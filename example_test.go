// Copyright (C) 2017 ScyllaDB
// Use of this source code is governed by a ALv2-style
// license that can be found in the LICENSE file.

// +build all integration

package gocqlx_test

import (
	"testing"
	"time"

	"github.com/scylladb/gocqlx"
	. "github.com/scylladb/gocqlx/gocqlxtest"
	"github.com/scylladb/gocqlx/qb"
)

func TestExample(t *testing.T) {
	session := CreateSession(t)
	defer session.Close()

	const personSchema = `
CREATE TABLE IF NOT EXISTS gocqlx_test.person (
    first_name text,
    last_name text,
    email list<text>,
    PRIMARY KEY(first_name, last_name)
)`

	if err := session.ExecStmt(personSchema); err != nil {
		t.Fatal("create table:", err)
	}

	// Person represents a row in person table.
	// Field names are converted to camel case by default, no need to add special tags.
	// If you want to disable a field add `db:"-"` tag, it will not be persisted.
	type Person struct {
		FirstName string
		LastName  string
		Email     []string
	}

	p := Person{
		"Patricia",
		"Citizen",
		[]string{"patricia.citzen@gocqlx_test.com"},
	}

	// Insert, bind data from struct.
	{
		stmt, names := qb.Insert("gocqlx_test.person").Columns("first_name", "last_name", "email").ToCql()
		q := session.Query(stmt, names).BindStruct(p)

		if err := q.ExecRelease(); err != nil {
			t.Fatal(err)
		}
	}

	// Insert with TTL and timestamp, bind data from struct and map.
	{
		stmt, names := qb.Insert("gocqlx_test.person").
			Columns("first_name", "last_name", "email").
			TTL(86400 * time.Second).
			Timestamp(time.Now()).
			ToCql()
		q := session.Query(stmt, names).BindStruct(p)

		if err := q.ExecRelease(); err != nil {
			t.Fatal(err)
		}
	}

	// Update email, bind data from struct.
	{
		p.Email = append(p.Email, "patricia1.citzen@gocqlx_test.com")

		stmt, names := qb.Update("gocqlx_test.person").
			Set("email").
			Where(qb.Eq("first_name"), qb.Eq("last_name")).
			ToCql()
		q := session.Query(stmt, names).BindStruct(p)

		if err := q.ExecRelease(); err != nil {
			t.Fatal(err)
		}
	}

	// Add email to a list.
	{
		stmt, names := qb.Update("gocqlx_test.person").
			AddNamed("email", "new_email").
			Where(qb.Eq("first_name"), qb.Eq("last_name")).
			ToCql()
		q := session.Query(stmt, names).BindStructMap(p, qb.M{
			"new_email": []string{"patricia2.citzen@gocqlx_test.com", "patricia3.citzen@gocqlx_test.com"},
		})

		if err := q.ExecRelease(); err != nil {
			t.Fatal(err)
		}
	}

	// Batch insert two rows in a single query.
	{
		i := qb.Insert("gocqlx_test.person").Columns("first_name", "last_name", "email")

		stmt, names := qb.Batch().
			AddWithPrefix("a", i).
			AddWithPrefix("b", i).
			ToCql()

		batch := struct {
			A Person
			B Person
		}{
			A: Person{
				"Igy",
				"Citizen",
				[]string{"igy.citzen@gocqlx_test.com"},
			},
			B: Person{
				"Ian",
				"Citizen",
				[]string{"ian.citzen@gocqlx_test.com"},
			},
		}
		q := session.Query(stmt, names).BindStruct(&batch)

		if err := q.ExecRelease(); err != nil {
			t.Fatal(err)
		}
	}

	// Get first result into a struct.
	{
		var p Person

		stmt, names := qb.Select("gocqlx_test.person").Where(qb.Eq("first_name")).ToCql()
		q := session.Query(stmt, names).BindMap(qb.M{
			"first_name": "Patricia",
		})

		if err := q.GetRelease(&p); err != nil {
			t.Fatal(err)
		}

		t.Log(p)
		// stdout: {Patricia Citizen [patricia.citzen@gocqlx_test.com patricia1.citzen@gocqlx_test.com patricia2.citzen@gocqlx_test.com patricia3.citzen@gocqlx_test.com]}
	}

	// Load all the results into a slice.
	{
		var people []Person

		stmt, names := qb.Select("gocqlx_test.person").Where(qb.In("first_name")).ToCql()
		q := session.Query(stmt, names).BindMap(qb.M{
			"first_name": []string{"Patricia", "Igy", "Ian"},
		})

		if err := q.SelectRelease(&people); err != nil {
			t.Fatal(err)
		}

		t.Log(people)
		// stdout: [{Ian Citizen [ian.citzen@gocqlx_test.com]} {Igy Citizen [igy.citzen@gocqlx_test.com]} {Patricia Citizen [patricia.citzen@gocqlx_test.com patricia1.citzen@gocqlx_test.com patricia2.citzen@gocqlx_test.com patricia3.citzen@gocqlx_test.com]}]
	}

	// Support for token based pagination.
	{
		p := &Person{
			"Ian",
			"Citizen",
			[]string{"ian.citzen@gocqlx_test.com"},
		}

		stmt, names := qb.Select("gocqlx_test.person").
			Columns("first_name").
			Where(qb.Token("first_name").Gt()).
			Limit(10).
			ToCql()
		q := session.Query(stmt, names).BindStruct(p)

		var people []Person
		if err := q.SelectRelease(&people); err != nil {
			t.Fatal(err)
		}

		t.Log(people)
		// [{Patricia  []} {Igy  []}]
	}

	// Support for named parameters in query string.
	{
		const query = "INSERT INTO gocqlx_test.person (first_name, last_name, email) VALUES (:first_name, :last_name, :email)"
		stmt, names, err := gocqlx.CompileNamedQuery([]byte(query))
		if err != nil {
			t.Fatal(err)
		}

		p := &Person{
			"Jane",
			"Citizen",
			[]string{"jane.citzen@gocqlx_test.com"},
		}
		q := session.Query(stmt, names).BindStruct(p)

		if err := q.ExecRelease(); err != nil {
			t.Fatal(err)
		}
	}
}
