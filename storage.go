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
//		"dbname="+dbname+" user=stags host=localhost sslmode=disable")
		"dbname=stags user=stags host=localhost sslmode=disable")
	if err != nil {
		log.Fatal("PostgreSQL connection error:", err)
	} else {
		db = &Database{ tmp, make(map[string]int32) }
		db.CreateTables()
		db.LoadTagCache()
	}

	return
}

// Create tables if needed.
func (db *Database) CreateTables() {
	row, err := db.Query("SELECT 1 FROM pg_type WHERE typname = 'dtype'")
	// if dtype doesn't exist, create it.
	// ADD NEW TYPES AT THE END IN PRODUCTION ENVIRONMENT
	if err == nil && !row.Next() {
		_, err = db.Query(`CREATE TYPE dtype AS ENUM
			(
				'string',
				'url',
				'pdf',
				'ps',
				'text'
			)
		`)
	}
	if err != nil {
		log.Fatal("Creation of dtype failed: ", err)
	}

	_, err = db.Query(`CREATE TABLE IF NOT EXISTS
		docs(
			id			SERIAL,
			name		TEXT,
			type		DTYPE,
			content		TEXT,
			PRIMARY KEY ("id")
		)
	`)
	if err != nil {
		log.Fatal("Creation of table docs failed: ", err)
	}

	_, err = db.Query(`CREATE TABLE IF NOT EXISTS
		tags(
			id			SERIAL,
			name		TEXT		UNIQUE,
			PRIMARY KEY ("id")
		)
	`)
	if err != nil {
		log.Fatal("Creation of table tags failed: ", err)
	}

	_, err = db.Query(`CREATE TABLE IF NOT EXISTS
		tagsdocs(
			idtag		INTEGER	REFERENCES	tags(id),
			iddoc		INTEGER	REFERENCES	docs(id)
		)
	`)
	if err != nil {
		log.Fatal("Creation of table tagsdocs failed: ", err)
	}
}

func (db *Database) LoadTagCache() {
	rows, err := db.Query("SELECT id, name FROM tags")
	if err != nil {
		log.Fatal("Cannot load tag cache")
	}

	for rows.Next() {
		var id		int32
		var name	string
		rows.Scan(&id, &name)
		db.tagcache[name] = id
	}
}

// Id is int32 so it matches INTEGER (SERIAL is INTEGER)
// cf. http://www.postgresql.org/docs/9.3/static/datatype-numeric.html
// Doc allows data exchange between DB and User.
// Capitalized name for JSON.
type Doc struct {
	Id			int32
	Name		string
	Type		string
	Content		string
	Tags		[]string
}

func (db *Database) GetDoc(id int32) (d Doc) {
	var tags string
	err := db.QueryRow(`SELECT docs.id, docs.name, docs.type,
			docs.content, string_agg(tags.name, U&'\001F') AS tags
		FROM
			tags, tagsdocs, docs
		WHERE
			tagsdocs.idtag = tags.id
		AND	tagsdocs.iddoc = docs.id
		AND	docs.id = $1
		GROUP BY docs.id`, id).Scan(&d.Id, &d.Name, &d.Type, &d.Content, &tags)
	if err != nil {
		log.Println("Cannot fetch doc", id, ":", err)
		d.Id = -1
	}

	d.Tags = strings.Split(tags, "\u001F")

	return
}

func (db *Database) AddTag(tag string) (id int32) {
	if strings.Contains(tag, "\u001F") {
		tag = strings.Replace(tag, "\u001F", "", -1)
	}

	id = -1
	if db.tagcache[tag] > 0 { return db.tagcache[tag] }

	err := db.QueryRow(`INSERT INTO tags(name) VALUES($1)
		RETURNING id`, tag).Scan(&id)
	if err != nil {
		log.Println("Cannot add tag:", err)
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
				log.Println("Cannot tag", id, "with", tag)
			}
		}
		
	}
}

func (db *Database) DelTags(id int32, tags []string) {
	log.Println("DelTags not implemented")
}

func (db *Database) UpdateContent(id int32, content string) {
	log.Println("UpdateContent not implemented")
}

func (db *Database) AddDoc(d Doc) (id int32) {
	id = -1
	err := db.QueryRow(`INSERT INTO docs(name, type, content)
		VALUES ($1, $2, $3)
		RETURNING id`, d.Name, d.Type, d.Content).Scan(&id)
	if err != nil {
		log.Println("Error while adding doc:", err)
	}

	if id != -1 {
		db.AddTags(id, d.Tags)
	}

	return
}

func (db *Database) DelDoc(id int32) {
	log.Println("DelDoc not implemented")
	return
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
func (db *Database) GetDocs(tags []string) (ds []Doc) {
	rows, err := db.Query(`SELECT docs.id
			FROM
				tags, tagsdocs, docs
			WHERE
				tagsdocs.idtag = tags.id
			AND	tagsdocs.iddoc = docs.id
			AND tags.name IN `+mkarray(tags)+`
			GROUP BY docs.id
			HAVING COUNT(docs.id) = $1`, len(tags))
	if err != nil {
		log.Println("Cannot fetch with tags ", tags, ":", err)
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

