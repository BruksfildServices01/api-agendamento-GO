package email

import "context"

type Attachment struct {
	Filename string
	Content  []byte
}

type Message struct {
	To          string
	Subject     string
	Body        string
	Attachments []Attachment
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}
