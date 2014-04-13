package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
)

const templatesdir = "templates"
const htmlindex = templatesdir+"/index.html"
const staticdir = "static"

var port = flag.String("port", "8080", "Listening HTTP port")

var db *Database

func senderror(w http.ResponseWriter, e int, msg string) {
	w.WriteHeader(e)
	fmt.Fprint(w, "{ \"Error\" : %d, \"Message\" : \"%s\" }", e, msg)
}

func senddoc(w http.ResponseWriter, d Doc) {
	res, err := json.Marshal(d)
	if err != nil {
		senderror(w, http.StatusInternalServerError, "bad marshaling")
	} else {
		w.Write(res)
	}
}

func delete(w http.ResponseWriter, r *http.Request) {
	log.Println("DELETE not implemented")
	senderror(w, http.StatusNotImplemented, "")
}

func get(w http.ResponseWriter, r *http.Request) {
	// drop trailing '/api/'
	id, err := strconv.ParseInt(r.URL.Path[5:], 10, 32)
	if err != nil {
		tags := strings.Split(r.URL.Path[5:], "\u001F")
		log.Println(tags)
		res, err := json.Marshal(db.GetDocs(tags))
		if err != nil {
			senderror(w, http.StatusInternalServerError, "bad marshaling")
		} else {
			w.Write(res)
		}
/*
		w.Write([]byte("{"))
		for _, d := range db.GetDocs(tags) {
			senddoc(w, d)
		}
		w.Write([]byte("}"))
*/
	} else {
		d := db.GetDoc(int32(id))
		if d.Id != -1 {
			senddoc(w, d)
		} else {
			senderror(w, http.StatusNotFound, "ID not found")
		}
	}
}

func post(w http.ResponseWriter, r *http.Request) {
	var d Doc

	buf := new(bytes.Buffer)
	buf.ReadFrom(r.Body)

	err := json.Unmarshal(buf.Bytes(), &d)
	if err != nil {
		log.Println("Wrong unmarshaling for ", buf.Bytes(), ":", err)
		senderror(w, http.StatusBadRequest, "Bad json")
		return
	}

	id := db.AddDoc(d)
	if id == -1 {
		senderror(w, http.StatusBadRequest, "Cannot add doc")
	} else {
		d.Id = id
		senddoc(w, d)
	}
}

func put(w http.ResponseWriter, r *http.Request) {
	log.Println("PUT not implemented")
	senderror(w, http.StatusNotImplemented, "")
}

func APIhandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "DELETE":
		delete(w, r)
	case "GET":
		get(w, r)
	case "POST":
		post(w, r)
	case "PUT":
		put(w, r)
	}
}

var index []byte

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Write(index)
	} else {
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("no."))
	}
}

func main() {
//	db = NewDB("stags")
	db = NewDB()

	var err error

	index, err = ioutil.ReadFile(htmlindex)
	if err != nil {
		log.Fatal("Can't load '"+htmlindex+"':", err)
	}

	http.HandleFunc("/", handler)
	http.HandleFunc("/api/", APIhandler)
	http.Handle("/static/",
		http.StripPrefix("/static/",
			http.FileServer(http.Dir(staticdir))))

	log.Print("Launching on http://localhost:"+*port)

	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
