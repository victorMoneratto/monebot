package main

import (
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// Database holds the necessary data for all persistent data operations
type Database struct {
	session  *mgo.Session
	commands *mgo.Collection
	packs    *mgo.Collection
}

// NewDatabase returns a new database connected through the connURI
func NewDatabase(connURI string) (*Database, error) {
	var db Database
	var err error
	db.session, err = mgo.Dial(connURI)
	if err != nil {
		return nil, err
	}

	db.commands = db.session.DB("").C("commands")
	db.packs = db.session.DB("").C("packs")

	return &db, nil
}

// Close the database session
func (db Database) Close() {
	db.session.Close()
}

// FindPack returns the default pack name for the chat
func (db Database) FindPack(chat int64) (string, error) {
	var pack Pack
	err := db.packs.Find(bson.M{"chats": chat}).One(&pack)
	if err != nil {
		return "", err
	}
	return pack.Name, nil
}

// FindCommand returns the one command filtered by the pack, name and numParams,
// or an error if not found
func (db Database) FindCommand(pack, name string, numParams int) (Command, error) {
	var c Command

	// Filter by name, numParams and packName, sort by pack descending and get the
	// first element (from specified pack first, if it exists, or from default pack)
	err := db.commands.Find(
		bson.M{"name": name,
			"answer.numParams": numParams,
			"$or": []bson.M{
				bson.M{"pack": pack},
				bson.M{"pack": ""},
			}}).Sort("-pack").One(&c)

	if err != nil {
		return c, err
	}

	return c, nil
}

// UpsertCommand updates or inserts the given command
func (db Database) UpsertCommand(c Command) error {
	_, err := db.commands.Upsert(
		bson.M{"pack": c.Pack,
			"name": c.Name,
			"answer.numParams": c.Answer.NumParams}, &c)

	return err
}