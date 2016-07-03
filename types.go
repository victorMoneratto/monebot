package main

import (
	"fmt"
	"time"
)

// Answer holds the possible messages the bot can send
type Answer struct {
	Text      string `bson:"text,omitempty"`
	NumParams int    `bson:"numParams"`
}

// Command holds the data about for persistent commands
type Command struct {
	Pack       string    `bson:"pack"`
	Name       string    `bson:"name"`
	Answer     Answer    `bson:"answer"`
	Time       time.Time `bson:"time"`
	Creator    string    `bson:"creator,omitempty"`
	NumChanged int       `bson:"numChanged,omitempty"`
}

// FullName returns the a string of the form <pack>.<name>
func (c Command) FullName() string {
	return fmt.Sprintf("%s.%s", c.Pack, c.Name)
}

// Pack holds a name for the pack and all chats that use it by default
type Pack struct {
	Name  string  `bson:"name"`
	Chats []int64 `bson:"chats"`
}
