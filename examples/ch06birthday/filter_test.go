// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch06birthday

import (
	"fmt"
	"testing"
	"time"
)

// Section 6.5: the chapter tests the filter exhaustively rather than
// randomly. For every year from 2018 to 2038 and every day of the year,
// three people born on each day of a leap year must each be greeted exactly
// once, with February 29 birthdays greeted on February 28 in other years.
func TestEveryBirthdayOnce(t *testing.T) {
	var people []Employee
	start := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC) // 2000 is a leap year
	for d := start; d.Year() == 2000; d = d.AddDate(0, 0, 1) {
		for i, year := range []int{2000, 1996, 1992} { // leap years, so February 29 exists
			people = append(people, Employee{
				LastName:    fmt.Sprintf("%02d%02d", d.Month(), d.Day()),
				FirstName:   fmt.Sprint(i),
				DateOfBirth: Date{year, int(d.Month()), d.Day()},
				Email:       fmt.Sprintf("%d-%02d-%02d-%d@example.com", year, d.Month(), d.Day(), i),
			})
		}
	}
	for year := 2018; year <= 2038; year++ {
		greeted := map[string]int{}
		for d := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC); d.Year() == year; d = d.AddDate(0, 0, 1) {
			today := Date{d.Year(), int(d.Month()), d.Day()}
			for _, e := range Birthdays(people, today) {
				greeted[e.Email]++
				dob := e.DateOfBirth
				sameDay := dob.Month == today.Month && dob.Day == today.Day
				feb29 := dob.Month == 2 && dob.Day == 29 && today.Month == 2 && today.Day == 28 && !IsLeapYear(year)
				if !sameDay && !feb29 {
					t.Errorf("%s greeted on %+v", e.Email, today)
				}
			}
		}
		for _, p := range people {
			if greeted[p.Email] != 1 {
				t.Errorf("%s greeted %d times in %d", p.Email, greeted[p.Email], year)
			}
		}
	}
}

// Section 6.8: the sample flat file through the whole program.
func TestGreetSample(t *testing.T) {
	flatFile := "last_name, first_name, date_of_birth, email\r\n" +
		"Doe, John, 1982/10/08, john.doe@foobar.com\r\n" +
		"Ann, Mary, 1975/09/11, mary.ann@foobar.com\r\n"
	var sent []Mail
	err := Greet(flatFile, Date{2026, 10, 8}, func(m Mail) error { sent = append(sent, m); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 || sent[0].To[0] != "john.doe@foobar.com" || sent[0].Body != "Happy birthday, dear John!" {
		t.Errorf("sent %+v", sent)
	}
}
