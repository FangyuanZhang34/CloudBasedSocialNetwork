package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/olivere/elastic"
)

//Define Elastic Search database and table
const (
	USER_INDEX = "user" // like database
	USER_TYPE  = "user" // like table
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Age      int64  `json:"age"`
	Gender   string `json:"gender"`
}

var mySigningKey = []byte("eramthgink")

func handlerLogin(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one log in request")
	w.Header().Set("Content-Type", "text/plain") //respond with token(a string)
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == "OPTIONS" {
		return
	}

	// decode Json format into User structure
	decoder := json.NewDecoder(r.Body)
	var user User
	if err := decoder.Decode(&user); err != nil {
		http.Error(w, "Cannot decode log in data from client", http.StatusBadRequest)
		fmt.Printf("Cannot decode login data from client %v. \n", err)
		return
	}

	if err := checkUser(user.Username, user.Password); err != nil {
		if err.Error() == "Wrong username or password!" {
			http.Error(w, "Wrong username or password!", http.StatusUnauthorized)
		} else {
			http.Error(w, "Failed to read from ElasticSearch", http.StatusInternalServerError)
		}
		return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": user.Username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString(mySigningKey)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		fmt.Printf("Failed to generate token %v.\n", err)
		return
	}

	w.Write([]byte(tokenString))

}

func handlerSignup(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received one sign up request")
	w.Header().Set("Content-Type", "text/plain") // http status 200/...
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method == "OPTIONS" {
		return
	}

	decoder := json.NewDecoder(r.Body)
	var user User
	if err := decoder.Decode(&user); err != nil {
		http.Error(w, "Cannot decode sign up data from client", http.StatusBadRequest)
		fmt.Printf("Cannot decode sign up data from client %v \n", err)
		return
	}

	if user.Username == "" || user.Password == "" || !regexp.MustCompile(`^[a-z0-9_]+$`).MatchString(user.Username) {
		http.Error(w, "Invalid username or password", http.StatusBadRequest)
		fmt.Printf("Invalid username or password")
		return
	}

	if err := addUser(user); err != nil {
		if err.Error() == "User already exists" {
			http.Error(w, "User already exists", http.StatusBadRequest)
		} else {
			http.Error(w, "Falsed to save to ElasticSearch", http.StatusInternalServerError)
		}
		return
	}
	w.Write([]byte("User added successfully"))

}

func checkUser(username, password string) error {
	client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false)) // ES status check in local IP addr
	if err != nil {
		return err
	}

	// select * from users where username = ?
	query := elastic.NewTermQuery("username", username)

	// do a search
	searchResult, err := client.Search().
		Index(USER_INDEX).
		Query(query).
		Pretty(true).
		Do(context.Background())
	if err != nil {
		return err
	}

	// convert result type into User and check if username and password match
	var utyp User
	for _, item := range searchResult.Each(reflect.TypeOf(utyp)) {
		if u, ok := item.(User); ok {
			if username == u.Username && password == u.Password {
				fmt.Printf("Log in as %s\n", username)
				return nil
			}
		}
	}

	return errors.New("Wrong username or password!")
}

func addUser(user User) error {
	client, err := elastic.NewClient(elastic.SetURL(ES_URL), elastic.SetSniff(false)) // ES status check in local IP addr
	if err != nil {
		return err
	}

	// insert ignore into users VALUES ...
	query := elastic.NewTermQuery("username", user.Username)
	// do a search
	searchResult, err := client.Search().
		Index(USER_INDEX).
		Query(query).
		Pretty(true).
		Do(context.Background())
	if err != nil {
		return err
	}

	// check if there is a search result existing
	if searchResult.TotalHits() > 0 {
		return errors.New("User already exists")
	}

	_, err = client.Index().
		Index(USER_INDEX).
		Type(USER_TYPE).
		Id(user.Username).
		BodyJson(user).
		Refresh("wait_for").
		Do(context.Background())

	if err != nil {
		return nil
	}

	fmt.Printf("User is added: %s\n", user.Username)
	return nil
}
