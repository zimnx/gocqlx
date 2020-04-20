// Copyright (C) 2017 ScyllaDB
// Use of this source code is governed by a ALv2-style
// license that can be found in the LICENSE file.

package gocqlx

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/gocql/gocql"
	"github.com/scylladb/go-reflectx"
)

// DefaultUnsafe enables the behavior of forcing the iterator to ignore
// missing fields for all queries. See Unsafe below for more information.
var DefaultUnsafe bool

// Iterx is a wrapper around gocql.Iter which adds struct scanning capabilities.
type Iterx struct {
	*gocql.Iter
	Mapper *reflectx.Mapper

	unsafe     bool
	structOnly bool
	started    bool
	err        error

	// Cache memory for a rows during iteration in StructScan.
	fields [][]int
	values []interface{}
}

// Unsafe forces the iterator to ignore missing fields. By default when scanning
// a struct if result row has a column that cannot be mapped to any destination
// field an error is reported. With unsafe such columns are ignored.
func (iter *Iterx) Unsafe() *Iterx {
	iter.unsafe = true
	return iter
}

// StructOnly forces the iterator to treat a single-argument struct as
// non-scannable. This is is useful if you need to scan a row into a struct
// that also implements gocql.UDTUnmarshaler or in rare cases gocql.Unmarshaler.
func (iter *Iterx) StructOnly() *Iterx {
	iter.structOnly = true
	return iter
}

// Get scans first row into a destination and closes the iterator.
//
// If the destination type is a struct pointer, then StructScan will be
// used.
// If the destination is some other type, then the row must only have one column
// which can scan into that type.
// This includes types that implement gocql.Unmarshaler and gocql.UDTUnmarshaler.
//
// If you'd like to treat a type that implements gocql.Unmarshaler or
// gocql.UDTUnmarshaler as an ordinary struct you should call
// StructOnly().Get(dest) instead.
//
// If no rows were selected, ErrNotFound is returned.
func (iter *Iterx) Get(dest interface{}) error {
	iter.scanAny(dest)
	iter.Close()

	return iter.checkErrAndNotFound()
}

// isScannable takes the reflect.Type and the actual dest value and returns
// whether or not it's Scannable. t is scannable if:
//   * ptr to t implements gocql.Unmarshaler or gocql.UDTUnmarshaler
//   * it is not a struct
//   * it has no exported fields
func (iter *Iterx) isScannable(t reflect.Type) bool {
	if ptr := reflect.PtrTo(t); ptr.Implements(unmarshallerInterface) || ptr.Implements(udtUnmarshallerInterface) {
		return true
	}
	if t.Kind() != reflect.Struct {
		return true
	}

	return len(iter.Mapper.TypeMap(t).Index) == 0
}

func (iter *Iterx) scanAny(dest interface{}) bool {
	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Ptr {
		iter.err = fmt.Errorf("expected a pointer but got %T", dest)
		return false
	}
	if value.IsNil() {
		iter.err = errors.New("expected a pointer but got nil")
		return false
	}

	base := reflectx.Deref(value.Type())
	scannable := iter.isScannable(base)

	if iter.structOnly && scannable {
		if base.Kind() == reflect.Struct {
			scannable = false
		} else {
			iter.err = structOnlyError(base)
			return false
		}
	}

	if scannable && len(iter.Columns()) > 1 {
		iter.err = fmt.Errorf("expected 1 column in result while scanning scannable type %s but got %d", base.Kind(), len(iter.Columns()))
		return false
	}

	if scannable {
		return iter.Scan(dest)
	}

	return iter.StructScan(dest)
}

// Select scans all rows into a destination, which must be a pointer to slice
// of any type, and closes the iterator.
//
// If the destination slice type is a struct, then StructScan will be used
// on each row.
// If the destination is some other type, then each row must only have one
// column which can scan into that type.
// This includes types that implement gocql.Unmarshaler and gocql.UDTUnmarshaler.
//
// If you'd like to treat a type that implements gocql.Unmarshaler or
// gocql.UDTUnmarshaler as an ordinary struct you should call
// StructOnly().Select(dest) instead.
//
// If no rows were selected, ErrNotFound is NOT returned.
func (iter *Iterx) Select(dest interface{}) error {
	iter.scanAll(dest)
	iter.Close()

	return iter.err
}

func (iter *Iterx) scanAll(dest interface{}) bool {
	value := reflect.ValueOf(dest)

	// json.Unmarshal returns errors for these
	if value.Kind() != reflect.Ptr {
		iter.err = fmt.Errorf("expected a pointer but got %T", dest)
		return false
	}
	if value.IsNil() {
		iter.err = errors.New("expected a pointer but got nil")
		return false
	}

	slice, err := baseType(value.Type(), reflect.Slice)
	if err != nil {
		iter.err = err
		return false
	}

	isPtr := slice.Elem().Kind() == reflect.Ptr
	base := reflectx.Deref(slice.Elem())
	scannable := iter.isScannable(base)

	if iter.structOnly && scannable {
		if base.Kind() == reflect.Struct {
			scannable = false
		} else {
			iter.err = structOnlyError(base)
			return false
		}
	}

	// if it's a base type make sure it only has 1 column;  if not return an error
	if scannable && len(iter.Columns()) > 1 {
		iter.err = fmt.Errorf("expected 1 column in result while scanning scannable type %s but got %d", base.Kind(), len(iter.Columns()))
		return false
	}

	var (
		alloc bool
		v     reflect.Value
		vp    reflect.Value
		ok    bool
	)
	for {
		// create a new struct type (which returns PtrTo) and indirect it
		vp = reflect.New(base)

		// scan into the struct field pointers
		if !scannable {
			ok = iter.StructScan(vp.Interface())
		} else {
			ok = iter.Scan(vp.Interface())
		}
		if !ok {
			break
		}

		// allocate memory for the page data
		if !alloc {
			v = reflect.MakeSlice(slice, 0, iter.NumRows())
			alloc = true
		}

		if isPtr {
			v = reflect.Append(v, vp)
		} else {
			v = reflect.Append(v, reflect.Indirect(vp))
		}
	}

	// update dest if allocated slice
	if alloc {
		reflect.Indirect(value).Set(v)
	}

	return true
}

// StructScan is like gocql.Iter.Scan, but scans a single row into a single
// struct. Use this and iterate manually when the memory load of Select() might
// be prohibitive. StructScan caches the reflect work of matching up column
// positions to fields to avoid that overhead per scan, which means it is not
// safe to run StructScan on the same Iterx instance with different struct
// types.
func (iter *Iterx) StructScan(dest interface{}) bool {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr {
		iter.err = errors.New("must pass a pointer, not a value, to StructScan destination")
		return false
	}

	if !iter.started {
		columns := columnNames(iter.Iter.Columns())
		m := iter.Mapper

		iter.fields = m.TraversalsByName(v.Type(), columns)
		// if we are not unsafe and are missing fields, return an error
		if !iter.unsafe {
			if f, err := missingFields(iter.fields); err != nil {
				iter.err = fmt.Errorf("missing destination name %q in %T", columns[f], dest)
				return false
			}
		}
		iter.values = make([]interface{}, len(columns))
		iter.started = true
	}

	err := fieldsByTraversal(v, iter.fields, iter.values, true)
	if err != nil {
		iter.err = err
		return false
	}
	// scan into the struct field pointers and append to our results
	return iter.Iter.Scan(iter.values...)
}

func columnNames(ci []gocql.ColumnInfo) []string {
	r := make([]string, len(ci))
	for i, column := range ci {
		r[i] = column.Name
	}
	return r
}

// Close closes the iterator and returns any errors that happened during
// the query or the iteration.
func (iter *Iterx) Close() error {
	err := iter.Iter.Close()
	if iter.err == nil {
		iter.err = err
	}
	return iter.err
}

// checkErrAndNotFound handle error and NotFound in one method.
func (iter *Iterx) checkErrAndNotFound() error {
	if iter.err != nil {
		return iter.err
	} else if iter.Iter.NumRows() == 0 {
		return gocql.ErrNotFound
	}
	return nil
}
