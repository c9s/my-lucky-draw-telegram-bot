package main

import (
	"github.com/google/uuid"

	tb "gopkg.in/tucnak/telebot.v2"
)

var markdownOption = &tb.SendOptions{
	ParseMode: "Markdown",
}

// newUUID 生成一个UUID
func newUUID() string {
	v4 := uuid.Must(uuid.NewUUID())
	return v4.String()
}
