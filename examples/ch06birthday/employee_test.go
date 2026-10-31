package ch06birthday

import (
	"fmt"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// The generators of Section 6.6: records as the flat file writes them, with
// a space after each comma and a zero-padded date.
var (
	whitespacedText = pbt.Map(field, func(s string) string { return " " + s })
	textDate        = pbt.Custom("date", func(t *pbt.T) string {
		return fmt.Sprintf(" %d/%02d/%02d", pbt.IntRange(1900, 2020).Draw(t, ""), pbt.IntRange(1, 12).Draw(t, ""), pbt.IntRange(1, 31).Draw(t, ""))
	})
	rawEmployeeMap = pbt.Custom("employee", func(t *pbt.T) map[string]string {
		return map[string]string{
			"last_name":      field.Draw(t, ""),
			" first_name":    whitespacedText.Draw(t, ""),
			" email":         whitespacedText.Draw(t, ""),
			" date_of_birth": textDate.Draw(t, ""),
		}
	})
	rawEmployees = pbt.SliceOf(rawEmployeeMap)
)

// prop_fix_csv_leading_space: no text field keeps its leading space.
func TestNoLeadingSpaces(t *testing.T) {
	pbt.Check(t, pbt.ForAll(rawEmployees, func(t *pbt.T, rows []map[string]string) {
		employees, err := FromCSV(Encode(rows))
		if err != nil {
			t.Fatalf("FromCSV: %v", err)
		}
		for _, e := range employees {
			for _, s := range []string{e.LastName, e.FirstName, e.Email} {
				if strings.HasPrefix(s, " ") {
					t.Fatalf("field %q keeps its leading space", s)
				}
			}
		}
	}))
}

// prop_fix_csv_date_of_birth: the date of birth is parsed into numbers.
func TestDateOfBirthIsParsed(t *testing.T) {
	pbt.Check(t, pbt.ForAll(rawEmployees, func(t *pbt.T, rows []map[string]string) {
		employees, err := FromCSV(Encode(rows))
		if err != nil {
			t.Fatalf("FromCSV: %v", err)
		}
		for _, e := range employees {
			d := e.DateOfBirth
			if d.Year < 1900 || d.Year > 2020 || d.Month < 1 || d.Month > 12 || d.Day < 1 || d.Day > 31 {
				t.Fatalf("date of birth %+v is out of range", d)
			}
		}
	}))
}

// prop_handle_access: filtering and the accessors never crash.
func TestFilterAndAccessors(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		rows := rawEmployees.Draw(t, "rows")
		today := Date{pbt.IntRange(2000, 2030).Draw(t, "year"), pbt.IntRange(1, 12).Draw(t, "month"), pbt.IntRange(1, 31).Draw(t, "day")}
		employees, err := FromCSV(Encode(rows))
		if err != nil {
			t.Fatalf("FromCSV: %v", err)
		}
		greeted := Birthdays(employees, today)
		t.Classify(len(greeted) > 0, "someone has a birthday")
		for _, e := range greeted {
			if e.DateOfBirth.Month != today.Month && !(today.Month == 2 && e.DateOfBirth.Month == 2) {
				t.Fatalf("%+v greeted on %+v", e, today)
			}
			_ = Full(e)
		}
	})
}

// Section 6.7: the body contains the first name.
func TestTemplate(t *testing.T) {
	employee := pbt.Custom("employee", func(t *pbt.T) Employee {
		return Employee{
			LastName:    pbt.StringN(1, 10).Draw(t, ""),
			FirstName:   pbt.StringN(1, 10).Draw(t, ""),
			DateOfBirth: Date{pbt.IntRange(1900, 2020).Draw(t, ""), pbt.IntRange(1, 12).Draw(t, ""), pbt.IntRange(1, 31).Draw(t, "")},
			Email:       pbt.StringN(1, 10).Draw(t, ""),
		}
	})
	pbt.Check(t, pbt.ForAll(employee, func(t *pbt.T, e Employee) {
		m := Full(e)
		if !strings.Contains(m.Body, e.FirstName) || m.Subject != "Happy birthday!" || len(m.To) != 1 || m.To[0] != e.Email {
			t.Fatalf("mail %+v does not fit %+v", m, e)
		}
	}))
}
