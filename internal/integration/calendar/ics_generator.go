package calendar

import (
	"bytes"
	"fmt"
	"time"
)

type ICSGenerator struct{}

func NewICSGenerator() *ICSGenerator {
	return &ICSGenerator{}
}

func (g *ICSGenerator) Generate(
	uid string,
	title string,
	description string,
	start time.Time,
	end time.Time,
	timezone string,
	cancelled bool,
) ([]byte, error) {

	status := "CONFIRMED"
	method := "REQUEST"

	if cancelled {
		status = "CANCELLED"
		method = "CANCEL"
	}

	buf := bytes.NewBuffer(nil)

	fmt.Fprintf(buf, "BEGIN:VCALENDAR\r\n")
	fmt.Fprintf(buf, "VERSION:2.0\r\n")
	fmt.Fprintf(buf, "PRODID:-//Barber Scheduler//EN\r\n")
	fmt.Fprintf(buf, "METHOD:%s\r\n", method)

	fmt.Fprintf(buf, "BEGIN:VEVENT\r\n")
	fmt.Fprintf(buf, "UID:%s\r\n", uid)
	fmt.Fprintf(buf, "DTSTAMP:%s\r\n", time.Now().UTC().Format("20060102T150405Z"))
	fmt.Fprintf(buf, "DTSTART:%s\r\n", start.UTC().Format("20060102T150405Z"))
	fmt.Fprintf(buf, "DTEND:%s\r\n", end.UTC().Format("20060102T150405Z"))
	fmt.Fprintf(buf, "SUMMARY:%s\r\n", title)
	fmt.Fprintf(buf, "DESCRIPTION:%s\r\n", description)
	fmt.Fprintf(buf, "STATUS:%s\r\n", status)
	fmt.Fprintf(buf, "END:VEVENT\r\n")

	fmt.Fprintf(buf, "END:VCALENDAR\r\n")

	return buf.Bytes(), nil
}
