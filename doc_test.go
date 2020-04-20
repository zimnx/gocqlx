// Copyright (C) 2017 ScyllaDB
// Use of this source code is governed by a ALv2-style
// license that can be found in the LICENSE file.

package gocqlx_test

import (
	"github.com/gocql/gocql"
	"github.com/scylladb/gocqlx"
	"github.com/scylladb/gocqlx/qb"
)

func ExampleSession() {
	cluster := gocql.NewCluster("host")
	session, err := gocqlx.WrapSession(cluster.CreateSession())
	if err != nil {
		// handle error
	}

	builder := qb.Select("foo")
	session.Query(builder.ToCql())
}
