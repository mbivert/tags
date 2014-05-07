package main

import (
	"errors"
	"github.com/gorilla/securecookie"
	"io/ioutil"
	"net/http"
	"strings"

//	"log"
)

const (
	authserver	=	"http://localhost:8080/"
	key = "4iXZNYbRYL6DbB4SHHyq6EtpEVCBvMebiUDfjPU1KeNd5Rv6BWVC85HKTmqcvKZJ"
	cname = "thipi-token"
)

var hashKey = []byte(securecookie.GenerateRandomKey(32))
var blockKey = []byte(securecookie.GenerateRandomKey(32))
var s = securecookie.New(hashKey, blockKey)

// Clean name for safe mkdir
func cleanName(name string) string {
	return strings.Join(strings.FieldsFunc(name, func (r rune) bool {
		return r == '/' || r == '.'
	}), "")
}

func mkr(descr string) (string, error) {
	resp, err := http.Get(authserver+"/api/"+descr+"&key="+key)
	// XXX make sure err doesn't embed sensible data (eg. key…)
	if err != nil { return "", err }

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil { return "", err }

	return string(body), nil
}

func update() {
}

func alogin(login string) (string, error) {
	return mkr("login?login="+login)
}

func chain(token string) (string, error) {
	return mkr("chain?token="+token)
}

func info(token string) (string, error) {
	return mkr("info?token="+token)
}

// retrieve token; chain it to the auth server
// return data stored in cookie or err
func checkToken(w http.ResponseWriter, r *http.Request) (string, error) {
	token, d, err := getToken(r)
	if err == nil { token, err = chain(token) }
	if err != nil { LogError(err); return "", err }

	if token == "ko" {
		return "", errors.New("bad token")
	}

	// previous token was valid, set new token
	err = setToken(w, token, d)

	return d, err
}

func setToken(w http.ResponseWriter, token, d string) error {
	value := map[string]string{
		"token"	:	token,
		"data"	:	d,
     	}
  
	if encoded, err := s.Encode(cname, value); err == nil {
		cookie := &http.Cookie{
			Name:	cname,
			Value:	encoded,
			Path:	"/",
		}
		http.SetCookie(w, cookie)
	} else {
		return err
	}

	return nil
}

func unsetToken(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:	cname,
		Value:	"",
		Path:	"/",
		MaxAge:	-1,
	}
	http.SetCookie(w, cookie)
}

func getToken(r *http.Request) (token, d string, err error) {
	if cookie, err := r.Cookie(cname); err == nil {
		value := map[string]string{}

		if err = s.Decode(cname, cookie.Value, &value); err == nil {
			return value["token"], value["data"], nil
		}
	}

	return "", "", err
}
