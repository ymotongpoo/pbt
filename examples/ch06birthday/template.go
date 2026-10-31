package ch06birthday

import "fmt"

// Body renders the greeting of the kata.
func Body(e Employee) string {
	return fmt.Sprintf("Happy birthday, dear %s!", e.FirstName)
}

// Mail is a message ready to send.
type Mail struct {
	To      []string
	Subject string
	Body    string
}

// Full renders the complete mail for an employee.
func Full(e Employee) Mail {
	return Mail{To: []string{e.Email}, Subject: "Happy birthday!", Body: Body(e)}
}

// Greet is the plumbing of Section 6.8: it reads the flat file, filters the
// employees whose birthday is today, and hands each mail to send. Reading
// the file, looking up the date and sending mail stay at the edges, which is
// what makes the rest testable without side effects.
func Greet(flatFile string, today Date, send func(Mail) error) error {
	employees, err := FromCSV(flatFile)
	if err != nil {
		return err
	}
	for _, e := range Birthdays(employees, today) {
		if err := send(Full(e)); err != nil {
			return err
		}
	}
	return nil
}
