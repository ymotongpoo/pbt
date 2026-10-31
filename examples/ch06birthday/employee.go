package ch06birthday

import (
	"fmt"
	"strconv"
	"strings"
)

// Date is a calendar date.
type Date struct{ Year, Month, Day int }

// Employee is one record of the flat file.
type Employee struct {
	LastName    string
	FirstName   string
	DateOfBirth Date
	Email       string
}

// FromCSV parses the flat file. Keys and values are stripped of leading
// spaces, since the file writes ", " after each comma, and the date of
// birth is parsed from YYYY/MM/DD. A field that is missing from a record is
// left empty, which is the chapter's undefined.
func FromCSV(s string) ([]Employee, error) {
	rows, err := Decode(s)
	if err != nil {
		return nil, err
	}
	employees := make([]Employee, 0, len(rows))
	for _, row := range rows {
		clean := make(map[string]string, len(row))
		for k, v := range row {
			clean[strings.TrimLeft(k, " ")] = strings.TrimLeft(v, " ")
		}
		dob, err := parseDate(clean["date_of_birth"])
		if err != nil {
			return nil, err
		}
		employees = append(employees, Employee{
			LastName:    clean["last_name"],
			FirstName:   clean["first_name"],
			DateOfBirth: dob,
			Email:       clean["email"],
		})
	}
	return employees, nil
}

func parseDate(s string) (Date, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 3 {
		return Date{}, fmt.Errorf("date %q is not YYYY/MM/DD", s)
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return Date{}, fmt.Errorf("date %q: %v", s, err)
		}
		nums[i] = n
	}
	return Date{nums[0], nums[1], nums[2]}, nil
}

// IsLeapYear reports whether a year has a February 29.
func IsLeapYear(y int) bool {
	return y%4 == 0 && (y%100 != 0 || y%400 == 0)
}

// Birthdays returns the employees whose birthday falls on the given date.
// On February 28 of a year without a February 29, it also returns those
// born on February 29, so that they are greeted every year.
func Birthdays(employees []Employee, today Date) []Employee {
	var out []Employee
	for _, e := range employees {
		dob := e.DateOfBirth
		switch {
		case dob.Month == today.Month && dob.Day == today.Day:
			out = append(out, e)
		case today.Month == 2 && today.Day == 28 && !IsLeapYear(today.Year) && dob.Month == 2 && dob.Day == 29:
			out = append(out, e)
		}
	}
	return out
}
