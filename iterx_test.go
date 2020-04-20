// Copyright (C) 2017 ScyllaDB
// Use of this source code is governed by a ALv2-style
// license that can be found in the LICENSE file.

// +build all integration

package gocqlx_test

import (
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gocql/gocql"
	"github.com/scylladb/gocqlx"
	. "github.com/scylladb/gocqlx/gocqlxtest"
	"github.com/scylladb/gocqlx/qb"
	"gopkg.in/inf.v0"
)

type FullName struct {
	FirstName string
	LastName  string
}

func (n FullName) MarshalCQL(info gocql.TypeInfo) ([]byte, error) {
	return []byte(n.FirstName + " " + n.LastName), nil
}

func (n *FullName) UnmarshalCQL(info gocql.TypeInfo, data []byte) error {
	t := strings.SplitN(string(data), " ", 2)
	n.FirstName, n.LastName = t[0], t[1]
	return nil
}

type FullNameUDT struct {
	FirstName string
	LastName  string
}

func (n FullNameUDT) MarshalUDT(name string, info gocql.TypeInfo) ([]byte, error) {
	f := gocqlx.DefaultMapper.FieldByName(reflect.ValueOf(n), name)
	return gocql.Marshal(info, f.Interface())
}

func (n *FullNameUDT) UnmarshalUDT(name string, info gocql.TypeInfo, data []byte) error {
	f := gocqlx.DefaultMapper.FieldByName(reflect.ValueOf(n), name)
	return gocql.Unmarshal(info, data, f.Addr().Interface())
}

func TestStruct(t *testing.T) {
	session := CreateSession(t)
	defer session.Close()

	if err := session.ExecStmt(`CREATE TYPE gocqlx_test.FullName (first_Name text, last_name text)`); err != nil {
		t.Fatal("create type:", err)
	}

	if err := session.ExecStmt(`CREATE TABLE gocqlx_test.struct_table (
			testuuid       timeuuid PRIMARY KEY,
			testtimestamp  timestamp,
			testvarchar    varchar,
			testbigint     bigint,
			testblob       blob,
			testbool       boolean,
			testfloat      float,
			testdouble     double,
			testint        int,
			testdecimal    decimal,
			testlist       list<text>,
			testset        set<int>,
			testmap        map<varchar, varchar>,
			testvarint     varint,
			testinet       inet,
			testcustom     text,
			testudt        gocqlx_test.FullName
		)`); err != nil {
		t.Fatal("create table:", err)
	}

	type StructTable struct {
		Testuuid      gocql.UUID
		Testvarchar   string
		Testbigint    int64
		Testtimestamp time.Time
		Testblob      []byte
		Testbool      bool
		Testfloat     float32
		Testdouble    float64
		Testint       int
		Testdecimal   *inf.Dec
		Testlist      []string
		Testset       []int
		Testmap       map[string]string
		Testvarint    *big.Int
		Testinet      string
		Testcustom    FullName
		Testudt       FullNameUDT
	}

	bigInt := new(big.Int)
	if _, ok := bigInt.SetString("830169365738487321165427203929228", 10); !ok {
		t.Fatal("failed setting bigint by string")
	}

	m := StructTable{
		Testuuid:      gocql.TimeUUID(),
		Testvarchar:   "Test VarChar",
		Testbigint:    time.Now().Unix(),
		Testtimestamp: time.Now().Truncate(time.Millisecond).UTC(),
		Testblob:      []byte("test blob"),
		Testbool:      true,
		Testfloat:     float32(4.564),
		Testdouble:    float64(4.815162342),
		Testint:       2343,
		Testdecimal:   inf.NewDec(100, 0),
		Testlist:      []string{"quux", "foo", "bar", "baz", "quux"},
		Testset:       []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
		Testmap:       map[string]string{"field1": "val1", "field2": "val2", "field3": "val3"},
		Testvarint:    bigInt,
		Testinet:      "213.212.2.19",
		Testcustom:    FullName{FirstName: "John", LastName: "Doe"},
		Testudt:       FullNameUDT{FirstName: "John", LastName: "Doe"},
	}

	const stmt = `INSERT INTO struct_table (testuuid, testtimestamp, testvarchar, testbigint, testblob, testbool, testfloat,testdouble, testint, testdecimal, testlist, testset, testmap, testvarint, testinet, testcustom, testudt) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	if err := session.Query(stmt, nil).Bind(
		m.Testuuid,
		m.Testtimestamp,
		m.Testvarchar,
		m.Testbigint,
		m.Testblob,
		m.Testbool,
		m.Testfloat,
		m.Testdouble,
		m.Testint,
		m.Testdecimal,
		m.Testlist,
		m.Testset,
		m.Testmap,
		m.Testvarint,
		m.Testinet,
		m.Testcustom,
		m.Testudt).ExecRelease(); err != nil {
		t.Fatal("insert:", err)
	}

	t.Run("get", func(t *testing.T) {
		var v StructTable
		if err := session.Query(`SELECT * FROM struct_table`, nil).Get(&v); err != nil {
			t.Fatal("get failed", err)
		}

		if !reflect.DeepEqual(m, v) {
			t.Fatal("not equals")
		}
	})

	t.Run("select", func(t *testing.T) {
		var v []StructTable
		if err := session.Query(`SELECT * FROM struct_table`, nil).Select(&v); err != nil {
			t.Fatal("select failed", err)
		}

		if len(v) != 1 {
			t.Fatal("select unexpected number of rows", len(v))
		}

		if !reflect.DeepEqual(m, v[0]) {
			t.Fatal("not equals")
		}
	})

	t.Run("select ptr", func(t *testing.T) {
		var v []*StructTable
		if err := session.Query(`SELECT * FROM struct_table`, nil).Select(&v); err != nil {
			t.Fatal("select failed", err)
		}

		if len(v) != 1 {
			t.Fatal("select unexpected number of rows", len(v))
		}

		if !reflect.DeepEqual(&m, v[0]) {
			t.Fatal("not equals")
		}
	})
}

func TestScannable(t *testing.T) {
	session := CreateSession(t)
	defer session.Close()
	if err := session.ExecStmt(`CREATE TABLE gocqlx_test.scannable_table (testfullname text PRIMARY KEY)`); err != nil {
		t.Fatal("create table:", err)
	}
	m := FullName{"John", "Doe"}

	if err := session.Query(`INSERT INTO scannable_table (testfullname) values (?)`, nil).Bind(m).Exec(); err != nil {
		t.Fatal("insert:", err)
	}

	t.Run("get", func(t *testing.T) {
		var v FullName
		if err := session.Query(`SELECT testfullname FROM scannable_table`, nil).Get(&v); err != nil {
			t.Fatal("get failed", err)
		}

		if !reflect.DeepEqual(m, v) {
			t.Fatal("not equals")
		}
	})

	t.Run("select", func(t *testing.T) {
		var v []FullName
		if err := session.Query(`SELECT testfullname FROM scannable_table`, nil).Select(&v); err != nil {
			t.Fatal("select failed", err)
		}

		if len(v) != 1 {
			t.Fatal("select unexpected number of rows", len(v))
		}

		if !reflect.DeepEqual(m, v[0]) {
			t.Fatal("not equals")
		}
	})

	t.Run("select ptr", func(t *testing.T) {
		var v []*FullName
		if err := session.Query(`SELECT testfullname FROM scannable_table`, nil).Select(&v); err != nil {
			t.Fatal("select failed", err)
		}

		if len(v) != 1 {
			t.Fatal("select unexpected number of rows", len(v))
		}

		if !reflect.DeepEqual(&m, v[0]) {
			t.Fatal("not equals")
		}
	})
}

func TestStructOnly(t *testing.T) {
	session := CreateSession(t)
	defer session.Close()
	if err := session.ExecStmt(`CREATE TABLE gocqlx_test.struct_only_table (first_name text, last_name text, PRIMARY KEY (first_name, last_name))`); err != nil {
		t.Fatal("create table:", err)
	}

	m := FullName{"John", "Doe"}

	if err := session.Query(`INSERT INTO struct_only_table (first_name, last_name) values (?, ?)`, nil).Bind(m.FirstName, m.LastName).Exec(); err != nil {
		t.Fatal("insert:", err)
	}

	t.Run("get", func(t *testing.T) {
		var v FullName
		if err := session.Query(`SELECT first_name, last_name FROM struct_only_table`, nil).Iter().StructOnly().Get(&v); err != nil {
			t.Fatal("get failed", err)
		}

		if !reflect.DeepEqual(m, v) {
			t.Fatal("not equals")
		}
	})

	t.Run("select", func(t *testing.T) {
		var v []FullName
		if err := session.Query(`SELECT first_name, last_name FROM struct_only_table`, nil).Iter().StructOnly().Select(&v); err != nil {
			t.Fatal("select failed", err)
		}

		if len(v) != 1 {
			t.Fatal("select unexpected number of rows", len(v))
		}

		if !reflect.DeepEqual(m, v[0]) {
			t.Fatal("not equals")
		}
	})

	t.Run("select ptr", func(t *testing.T) {
		var v []*FullName
		if err := session.Query(`SELECT first_name, last_name FROM struct_only_table`, nil).Iter().StructOnly().Select(&v); err != nil {
			t.Fatal("select failed", err)
		}

		if len(v) != 1 {
			t.Fatal("select unexpected number of rows", len(v))
		}

		if !reflect.DeepEqual(&m, v[0]) {
			t.Fatal("not equals")
		}
	})

	t.Run("get error", func(t *testing.T) {
		var v FullName
		err := session.Query(`SELECT first_name, last_name FROM struct_only_table`, nil).Get(&v)
		if err == nil || !strings.HasPrefix(err.Error(), "expected 1 column in result") {
			t.Fatal("get expected validation error got", err)
		}
	})

	t.Run("select error", func(t *testing.T) {
		var v []FullName
		err := session.Query(`SELECT first_name, last_name FROM struct_only_table`, nil).Select(&v)
		if err == nil || !strings.HasPrefix(err.Error(), "expected 1 column in result") {
			t.Fatal("select expected validation error got", err)
		}
	})
}

func TestStructOnlyUDT(t *testing.T) {
	session := CreateSession(t)
	defer session.Close()
	if err := session.ExecStmt(`CREATE TABLE gocqlx_test.struct_only_udt_table (first_name text, last_name text, PRIMARY KEY (first_name, last_name))`); err != nil {
		t.Fatal("create table:", err)
	}

	m := FullNameUDT{"John", "Doe"}

	if err := session.Query(`INSERT INTO struct_only_udt_table (first_name, last_name) values (?, ?)`, nil).Bind(m.FirstName, m.LastName).Exec(); err != nil {
		t.Fatal("insert:", err)
	}

	t.Run("get", func(t *testing.T) {
		var v FullNameUDT
		if err := session.Query(`SELECT first_name, last_name FROM struct_only_udt_table`, nil).Iter().StructOnly().Get(&v); err != nil {
			t.Fatal("get failed", err)
		}

		if !reflect.DeepEqual(m, v) {
			t.Fatal("not equals")
		}
	})

	t.Run("select", func(t *testing.T) {
		var v []FullNameUDT
		if err := session.Query(`SELECT first_name, last_name FROM struct_only_udt_table`, nil).Iter().StructOnly().Select(&v); err != nil {
			t.Fatal("select failed", err)
		}

		if len(v) != 1 {
			t.Fatal("select unexpected number of rows", len(v))
		}

		if !reflect.DeepEqual(m, v[0]) {
			t.Fatal("not equals")
		}
	})

	t.Run("select ptr", func(t *testing.T) {
		var v []*FullNameUDT
		if err := session.Query(`SELECT first_name, last_name FROM struct_only_udt_table`, nil).Iter().StructOnly().Select(&v); err != nil {
			t.Fatal("select failed", err)
		}

		if len(v) != 1 {
			t.Fatal("select unexpected number of rows", len(v))
		}

		if !reflect.DeepEqual(&m, v[0]) {
			t.Fatal("not equals")
		}
	})

	t.Run("get error", func(t *testing.T) {
		var v FullNameUDT
		err := session.Query(`SELECT first_name, last_name FROM struct_only_udt_table`, nil).Get(&v)
		if err == nil || !strings.HasPrefix(err.Error(), "expected 1 column in result") {
			t.Fatal("get expected validation error got", err)
		}
	})

	t.Run("select error", func(t *testing.T) {
		var v []FullNameUDT
		err := session.Query(`SELECT first_name, last_name FROM struct_only_udt_table`, nil).Select(&v)
		if err == nil || !strings.HasPrefix(err.Error(), "expected 1 column in result") {
			t.Fatal("select expected validation error got", err)
		}
	})
}

func TestUnsafe(t *testing.T) {
	session := CreateSession(t)
	defer session.Close()
	if err := session.ExecStmt(`CREATE TABLE gocqlx_test.unsafe_table (testtext text PRIMARY KEY, testtextunbound text)`); err != nil {
		t.Fatal("create table:", err)
	}
	if err := session.Query(`INSERT INTO unsafe_table (testtext, testtextunbound) values (?, ?)`, nil).Bind("test", "test").Exec(); err != nil {
		t.Fatal("insert:", err)
	}

	type UnsafeTable struct {
		Testtext string
	}

	t.Run("safe get", func(t *testing.T) {
		var v UnsafeTable
		err := session.Query(`SELECT * FROM unsafe_table`, nil).Get(&v)
		if err == nil || err.Error() != "missing destination name \"testtextunbound\" in *gocqlx_test.UnsafeTable" {
			t.Fatal("expected ErrNotFound", "got", err)
		}
	})

	t.Run("safe select", func(t *testing.T) {
		var v []UnsafeTable
		err := session.Query(`SELECT * FROM unsafe_table`, nil).Select(&v)
		if err == nil || err.Error() != "missing destination name \"testtextunbound\" in *gocqlx_test.UnsafeTable" {
			t.Fatal("expected ErrNotFound", "got", err)
		}
		if cap(v) > 0 {
			t.Fatal("side effect alloc")
		}
	})

	t.Run("unsafe get", func(t *testing.T) {
		var v UnsafeTable
		err := session.Query(`SELECT * FROM unsafe_table`, nil).Iter().Unsafe().Get(&v)
		if err != nil {
			t.Fatal(err)
		}
		if v.Testtext != "test" {
			t.Fatal("get failed")
		}
	})

	t.Run("unsafe select", func(t *testing.T) {
		var v []UnsafeTable
		err := session.Query(`SELECT * FROM unsafe_table`, nil).Iter().Unsafe().Select(&v)
		if err != nil {
			t.Fatal(err)
		}
		if len(v) != 1 {
			t.Fatal("select failed")
		}
		if v[0].Testtext != "test" {
			t.Fatal("select failed")
		}
	})

	t.Run("DefaultUnsafe select", func(t *testing.T) {
		gocqlx.DefaultUnsafe = true
		defer func() {
			gocqlx.DefaultUnsafe = false
		}()
		var v []UnsafeTable
		err := session.Query(`SELECT * FROM unsafe_table`, nil).Iter().Select(&v)
		if err != nil {
			t.Fatal(err)
		}
		if len(v) != 1 {
			t.Fatal("select failed")
		}
		if v[0].Testtext != "test" {
			t.Fatal("select failed")
		}
	})
}

func TestNotFound(t *testing.T) {
	session := CreateSession(t)
	defer session.Close()
	if err := session.ExecStmt(`CREATE TABLE gocqlx_test.not_found_table (testtext text PRIMARY KEY)`); err != nil {
		t.Fatal("create table:", err)
	}

	type NotFoundTable struct {
		Testtext string
	}

	t.Run("get cql error", func(t *testing.T) {
		var v NotFoundTable
		err := session.Query(`SELECT * FROM not_found_table WRONG`, nil).RetryPolicy(nil).Get(&v)
		if err == nil || !strings.Contains(err.Error(), "WRONG") {
			t.Fatal(err)
		}
	})

	t.Run("get", func(t *testing.T) {
		var v NotFoundTable
		err := session.Query(`SELECT * FROM not_found_table`, nil).Get(&v)
		if err != gocql.ErrNotFound {
			t.Fatal("expected ErrNotFound", "got", err)
		}
	})

	t.Run("select cql error", func(t *testing.T) {
		var v []NotFoundTable
		err := session.Query(`SELECT * FROM not_found_table WRONG`, nil).RetryPolicy(nil).Select(&v)
		if err == nil || !strings.Contains(err.Error(), "WRONG") {
			t.Fatal(err)
		}
	})

	t.Run("select", func(t *testing.T) {
		var v []NotFoundTable
		err := session.Query(`SELECT * FROM not_found_table`, nil).Select(&v)
		if err != nil {
			t.Fatal(err)
		}
		if cap(v) > 0 {
			t.Fatal("side effect alloc")
		}
	})
}

func TestPaging(t *testing.T) {
	session := CreateSession(t)
	defer session.Close()
	if err := session.ExecStmt(`CREATE TABLE gocqlx_test.paging_table (id int PRIMARY KEY, val int)`); err != nil {
		t.Fatal("create table:", err)
	}
	if err := session.ExecStmt(`CREATE INDEX id_val_index ON gocqlx_test.paging_table (val)`); err != nil {
		t.Fatal("create index:", err)
	}

	q := session.Query(qb.Insert("gocqlx_test.paging_table").Columns("id", "val").ToCql())
	for i := 0; i < 5000; i++ {
		if err := q.Bind(i, i).Exec(); err != nil {
			t.Fatal(err)
		}
	}

	type Paging struct {
		ID  int
		Val int
	}

	t.Run("iter", func(t *testing.T) {
		stmt, names := qb.Select("gocqlx_test.paging_table").
			Where(qb.Lt("val")).
			AllowFiltering().
			Columns("id", "val").ToCql()
		iter := session.Query(stmt, names).Bind(100).PageSize(10).Iter()
		defer iter.Close()

		var cnt int
		for {
			p := &Paging{}
			if !iter.StructScan(p) {
				break
			}
			cnt++
		}
		if cnt != 100 {
			t.Fatal("expected 100", "got", cnt)
		}
	})
}
