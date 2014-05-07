package main

/*
postgres@earth:~$ createdb stags
postgres@earth:~$ psql stags
psql (9.3.4)
Type "help" for help.
stags=# CREATE ROLE stags PASSWORD 'stags' LOGIN;
CREATE ROLE
stags=# 
*/

import (
	_ "github.com/lib/pq"
	"database/sql"
	"errors"
	"log"
	"strings"
)

type Database struct {
	*sql.DB
	tagcache	map[string]int32
}

// XXX add password; check for clean user setup; check SSL
//func NewDB(dbname string) (db *Database) {
func NewDB() (db *Database) {
	tmp, err := sql.Open("postgres",
		"dbname=stags user=stags host=localhost sslmode=disable")
	if err != nil {
		LogFatal(err)
	} else {
		db = &Database{ tmp, make(map[string]int32) }
		db.Init()
	}

	return
}

func (db *Database) createFatal(descr string) {
	_, err := db.Query(descr)
	if err != nil {
		LogFatal(err)
	}
}

// Create tables if needed.
func (db *Database) createTables() {
	row, err := db.Query("SELECT 1 FROM pg_type WHERE typname = 'dtype'")
	// if dtype doesn't exist, create it.
	// XXX ADD NEW TYPES AT THE END IN PRODUCTION
	if err == nil && !row.Next() {
		db.createFatal(`CREATE TYPE dtype AS ENUM
			(
				'text',
				'url',
				'pdf',
				'ps'
			)
		`)
	}

	db.createFatal(`CREATE TABLE IF NOT EXISTS
		docs(
			id			SERIAL,
			name		TEXT,
			type		DTYPE,
			content		TEXT,
			uid			INT,
			PRIMARY KEY ("id")
		)
	`)

	db.createFatal(`CREATE TABLE IF NOT EXISTS
		tags(
			id			SERIAL,
			name		TEXT		UNIQUE,
			PRIMARY KEY ("id")
		)
	`)

	db.createFatal(`CREATE TABLE IF NOT EXISTS
		tagsdocs(
			idtag		INTEGER	REFERENCES	tags(id)	ON DELETE CASCADE,
			iddoc		INTEGER	REFERENCES	docs(id)	ON DELETE CASCADE
		)
	`)
}

func (db *Database) loadTagCache() {
	rows, err := db.Query("SELECT id, name FROM tags")
	if err != nil {
		LogFatal(errors.New("Cannot load tag cache"))
	}

	for rows.Next() {
		var id		int32
		var name	string
		rows.Scan(&id, &name)
		db.tagcache[name] = id
	}
}

func (db *Database) Init() {
	db.createTables()
	db.loadTagCache()
}

func (db *Database) HasOwner(id, uid int32) bool {
	err := db.QueryRow(`SELECT id FROM docs WHERE
		id = $1 AND uid = $2`, id, uid).Scan(&id)

	return err == nil
}

func (db *Database) GetDoc(id int32) (d Doc) {
	var tags string
	err := db.QueryRow(`SELECT docs.id, docs.name, docs.type,
			docs.content, docs.uid, string_agg(tags.name, U&'\001F') AS tags
		FROM
			tags, tagsdocs, docs
		WHERE
			tagsdocs.idtag = tags.id
		AND	tagsdocs.iddoc = docs.id
		AND	docs.id = $1
		GROUP BY docs.id`, id).Scan(&d.Id, &d.Name, &d.Type, &d.Content, &d.Uid, &tags)
	if err != nil {
		LogError(err)
		d.Id = -1
	} else {
		d.Tags = strings.Split(tags, TagSep)
	}

	return
}

func (db *Database) AddTag(tag string) (id int32) {
	if strings.Contains(tag, TagSep) {
		tag = strings.Replace(tag, TagSep, "", -1)
	}

	id = -1
	if db.tagcache[tag] > 0 { return db.tagcache[tag] }

	err := db.QueryRow(`INSERT INTO tags(name) VALUES($1)
		RETURNING id`, tag).Scan(&id)
	if err != nil {
		LogError(err)
	} else {
		db.tagcache[tag] = id
	}
	return
}

func (db *Database) AddTags(id int32, tags []string) {
	for _, tag := range tags {
		idtag := db.AddTag(tag)
		if idtag != -1 {
			_, err := db.Query(`INSERT into tagsdocs(idtag, iddoc)
				VALUES($1, $2)`, idtag, id)
			if err != nil {
				LogError(err)
			}
		}
		
	}
}

func (db *Database) DelTags(id int32, tags []string) {
	if _, err := db.Query(`DELETE FROM tagsdocs USING tags
		WHERE
			tagsdocs.idtag = tags.id
		AND	tagsdocs.iddoc = $1
		AND	tags.name IN `+mkarray(tags), id); err != nil {
			log.Println(err)
	}
}

func (db *Database) UpdateDoc(d *Doc) {
	old := db.GetDoc(d.Id)

	f := "("; v := "("

	if old.Name != d.Name {
		f += "name"
		v += "'"+d.Name+"'"
	}
	if old.Type != d.Type {
		f += ", type"
		v += ", '"+d.Type+"'"
	}
	if old.Content != d.Content {
		f += ", content"
		v += ", '"+d.Content+"'"
	}

	if v != "(" {
		f += ")"; v += ")"
		_, err := db.Query(`UPDATE docs SET `+f+` = `+v+`
			WHERE docs.id = $1`, d.Id)
		if err != nil {
			log.Println(err)
		}
	}

	if len(d.Tags) > 0 {
		db.DelTags(d.Id, old.Tags)
		db.AddTags(d.Id, d.Tags)
	}
}

func (db *Database) AddDoc(d *Doc) (id int32) {
	id = -1
	err := db.QueryRow(`INSERT INTO docs(name, type, content, uid)
		VALUES ($1, $2, $3, $4)
		RETURNING id`, d.Name, d.Type, d.Content, d.Uid).Scan(&id)
	if err != nil {
		LogError(err)
	}

	if id != -1 {
		db.AddTags(id, d.Tags)
	}

	return
}

func (db *Database) DelDoc(id int32) {
	if _, err := db.Query("DELETE FROM docs WHERE docs.id = $1", id); err != nil {
		log.Println(err)
	}
}

func mkarray(ss []string) (res string) {
	res = "("
	for i, s := range ss {
		res += "'"+s+"'"
		if i < len(ss)-1 { res += "," }
	}
	res += ")"
	return
}

// following advices from
// http://tagging.pui.ch/post/37027745720/tags-database-schemas
// upon filtering : fetch every item which contains all the mandatory
// tag, fetch them and do additional filtering here and not with SQL.
// XXX add a cache id<->tags to avoid request
func (db *Database) GetDocs(uid int32, tags []string) (ds []Doc) {
	if len(tags) == 0 { return }

	rows, err := db.Query(`SELECT docs.id
			FROM
				tags, tagsdocs, docs
			WHERE
				tagsdocs.idtag	= tags.id
			AND	tagsdocs.iddoc	= docs.id
			AND	(docs.uid		= $1
			OR	tags.name = ':public')
			AND tags.name IN `+mkarray(tags)+`
			GROUP BY docs.id
			HAVING COUNT(docs.id) = $2`, uid, len(tags))
	if err != nil {
		LogError(err)
		return
	}

	for rows.Next() {
		var id int32
		rows.Scan(&id)
		// XXX allows deeper filtering here.
		ds = append(ds, db.GetDoc(id))
	}

	return
}

