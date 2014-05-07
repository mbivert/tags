package main

import (
	"flag"
	"github.com/dchest/captcha"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
)

var port = flag.String("port", "8082", "Listening HTTP port")

var db *Database

var ltmpl = template.Must(
	template.New("login.html").ParseFiles("templates/login.html"))

var utmpl = template.Must(
	template.New("user.html").Funcs( template.FuncMap{
		"GetTags": func(tags []string) string {
			return strings.Join(tags, ", ")
		},
		// from rsc'eq
		"eq": func(x, y interface{}) bool {
			switch x := x.(type) {
			case string, int, int64, int32:
				return x == y
			}
			return false
		},
	}).ParseFiles("templates/user.html"))

func splitTags(tags string) []string {
	return strings.FieldsFunc(tags, func(r rune) bool {
		switch r {
		case []rune(TagSep)[0], ' ', ',', '\n', '\t':
			return true
		}
		return false
	})
}

func getType(c string) string {
	// XXX may regexp
	// "^((f|ht)tps?:// | [A-Z]:\\ | /)"
	switch {
	case strings.HasPrefix(c, "http://"),
		 strings.HasPrefix(c, "https?://"),
		 strings.HasPrefix(c, "ftp://"),
		 strings.HasPrefix(c, "ftps://"),
		 strings.HasPrefix(c, "C:"),
		 strings.HasPrefix(c, "/"):
		return "url"
	default:
		return "text"
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	writeFiles(w, "templates/header.html", GetNavbar(r),
		"templates/index.html", "templates/footer.html")
}

func getLogin(w http.ResponseWriter, r *http.Request) {
	writeFiles(w, "templates/header.html", "templates/navbar.html")

	d := struct { CaptchaId string }{ captcha.New() }

	if err := ltmpl.Execute(w, &d); err != nil {
		LogHttp(w, err); return
	}

	writeFiles(w, "templates/footer.html")
}

func postLogin(w http.ResponseWriter, r *http.Request) {
/*
	if !captcha.VerifyString(r.FormValue("captchaId"), r.FormValue("captchaRes")) {
		w.Write([]byte("<p>Bad captcha; try again. </p>"))
		return
	}
*/
	// login may be email, name or token
	login := r.FormValue("login");

	resp, err := alogin(login)
	if err != nil {
		LogHttp(w, err); return
	}

	switch resp {
	// received a valid token, effectively login the user
	case "ok":
		// fetch id from server
		udata, err := info(login)
		if err != nil { LogHttp(w, err); return }
		id := strings.Split(udata, "\n")[0]

		// generate a new token
		token, _ := chain(login)
		setToken(w, token, id)

		// everything went well, redirect
		http.Redirect(w, r, "/user/", http.StatusFound)

	// wrong data.
	case "ko":
		w.Write([]byte("<p>Wrong token/email/name; retry</p>"))

	// new token has been generated
	case "new":
		w.Write([]byte(`<p>Check your AAS account, and <a href="/login">login</a>!</p>`))
	}
}

func login(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getLogin(w, r)
	case "POST":
		postLogin(w, r)
	}
}

func logout(w http.ResponseWriter, r *http.Request) {
	unsetToken(w)

	http.Redirect(w, r, "/", http.StatusFound)
}

func user(w http.ResponseWriter, r *http.Request) {
	// check connection information; get id
	idstr, err := checkToken(w, r)
	if err != nil {
		w.Write([]byte(`<p>Please <a href="/login">login</a></p>.`))
		return
	}

	i, _ := strconv.ParseInt(idstr, 10, 32)
	uid := int32(i)

	writeFiles(w, "templates/header.html", "templates/navbar2.html")

	// fetch docs
	tags := r.FormValue("search")

	docs := db.GetDocs(uid, splitTags(tags))

	d := struct {
		Empty	Doc
		Docs	[]Doc
		Uid		int32
	} { Doc{ uid, "", "", "Some content", -1, []string{""} }, docs, uid }

	if err := utmpl.Execute(w, &d); err != nil {
		LogHttp(w, err); return
	}

	writeFiles(w, "templates/footer.html")
}

func add(w http.ResponseWriter, r *http.Request) {
	// check if connected
	idstr, err := checkToken(w, r)
	if err != nil {
		w.Write([]byte(`<p>Please <a href="/login">login</a></p>.`))
		return
	}

	i, _ := strconv.ParseInt(idstr, 10, 32)
	uid := int32(i)

	name := r.FormValue("name")
	tags := splitTags(r.FormValue("tags"))
	// XXX element not added (can't be retrieved)
	if len(tags) == 0 { return }

	content := strings.TrimSpace(r.FormValue("content"))
	typ := getType(content)

	id := db.AddDoc(&Doc{-1, name, typ, content, uid, tags })
	if id == -1 {
		// XXX element not added (stuff may already exists)
		return
	}

	http.Redirect(w, r, "/user/", http.StatusFound)
}

func edit(w http.ResponseWriter, r *http.Request) {/*
	// check if connected
	idstr, err := checkToken(w, r)
	if err != nil {
		w.Write([]byte(`<p>Please <a href="/login">login</a></p>.`))
		return
	}

	i, _ := strconv.ParseInt(idstr, 10, 32)
	uid := int32(i)
*/}

func main() {
	db = NewDB()

	http.HandleFunc("/", index)
	http.HandleFunc("/login/", login)
	http.HandleFunc("/logout/", logout)
	http.HandleFunc("/user/", user)
	http.HandleFunc("/add/", add)
	http.HandleFunc("/edit/", edit)

	// TODO automatically tag :bookmark for type=url
//	http.HandleFunc("/settings/", settings)

	// Captchas
	http.Handle("/captcha/",
		captcha.Server(captcha.StdWidth, captcha.StdHeight))

	http.Handle("/static/",
		http.StripPrefix("/static/",
			http.FileServer(http.Dir("static"))))

	log.Print("Launching on http://localhost:"+*port)

	log.Fatal(http.ListenAndServe(":"+*port, nil))
}
