package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Rsvp struct {
	Name, Email, Phone string
	WillAttend         bool
}

const pathToTemplate = "serverLearn/templates/"

var responses = make([]*Rsvp, 0, 10)
var templates = make(map[string]*template.Template, 3)

func loadTemplates() {
	templateNames := [5]string{"welcome", "form", "thanks", "sorry", "list"}
	for index, name := range templateNames {
		t, err := template.ParseFiles(pathToTemplate+"layout.html", pathToTemplate+name+".html")
		if err == nil {
			templates[name] = t
			fmt.Println("Loaded template", index, name)
		} else {
			panic(err)
		}
	}
}

func welcomeHandler(writer http.ResponseWriter, request *http.Request) {
	templates["welcome"].Execute(writer, nil)
}

func listHandler(writer http.ResponseWriter, request *http.Request) {
	templates["list"].Execute(writer, responses)
}

type formData struct {
	*Rsvp
	Errors []string
}

func validMailAddress(address string) bool {
	_, err := mail.ParseAddress(address)
	resp := false
	if err == nil {
		resp = true
	}
	return resp
}

func formHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		templates["form"].Execute(writer, formData{
			Rsvp: &Rsvp{}, Errors: []string{},
		})
	} else if request.Method == http.MethodPost {
		request.ParseForm()
		var responseData Rsvp
		errors := []string{}
		if validMailAddress(request.Form["email"][0]) {
			responseData = Rsvp{
				Name:       request.Form["name"][0],
				Email:      request.Form["email"][0],
				Phone:      request.Form["phone"][0],
				WillAttend: request.Form["willattend"][0] == "true",
			}
			if responseData.Name == "" {
				errors = append(errors, "Please enter your name")
			}
			if responseData.Email == "" {
				errors = append(errors, "Please enter your email address")
			}
			if responseData.Phone == "" {
				errors = append(errors, "Please enter your phone number")
			}
		} else {
			errors = append(errors, "Please enter your correct email address")
		}
		if len(errors) > 0 {
			templates["form"].Execute(writer, formData{
				Rsvp: &responseData, Errors: errors,
			})
		} else {
			responses = append(responses, &responseData)
			if responseData.WillAttend {
				writeInFile(responseData)
				templates["thanks"].Execute(writer, responseData.Name)
			} else {
				templates["sorry"].Execute(writer, responseData.Name)
			}
		}
	}
}
func writeInFile(data Rsvp) {
	filePath := "temp/test.txt"
	note := data.Name + " " + data.Email + " " + data.Phone + " created_at: " + time.Now().Format(time.RFC822)
	if _, err := os.Stat(filePath); err != nil {
		errors := os.WriteFile(filePath, []byte(note), 0666)
		if errors != nil {
			fmt.Println(errors)
		}
	} else {
		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			fmt.Println(err)
		}
		defer file.Close()
		file.WriteString("\n")
		file.WriteString(note)
	}
}

func request(apiKey string) {
	resp, err := http.Get("https://api.openweathermap.org/data/2.5/weather?id=479123&units=metric&appid=" + apiKey + "&lang=ru")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()
	bs := make([]byte, 1014)
	n, err := resp.Body.Read(bs)
	if err != nil {
		filePath := "temp/weather.txt"
		note := string(bs[:n])
		if _, err := os.Stat(filePath); err != nil {
			errors := os.WriteFile(filePath, []byte(note), 0666)
			if errors != nil {
				fmt.Println(errors)
			}
		}
	}
}

func readFromFile() string {
	f, err := os.Open("temp/apiKey.txt")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	wr := bytes.Buffer{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		wr.WriteString(sc.Text())
	}
	return wr.String()
}

var database *sql.DB

type Book struct {
	Id     int
	Title  string
	Author string
}

func sqlCon() []Book {
	db, err := sql.Open("mysql", "root:@/library")
	fmt.Println(err)
	books := []Book{}
	if err == nil {
		db.SetConnMaxLifetime(time.Minute * 3)
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(10)
		database = db
		rows, err := database.Query("select id, title, author from library.Books limit 10000")
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				book := Book{}
				err := rows.Scan(&book.Id, &book.Title, &book.Author)
				if err != nil {
					fmt.Println(err)
					continue
				}
				books = append(books, book)
			}
			return books
		}
	}
	return books
}

func writeInCsv(books []Book) {
	file, err := os.Create("temp/books.csv")
	if err == nil {
		defer file.Close()
		writer := csv.NewWriter(file)
		defer writer.Flush()
		for _, book := range books {
			var title []string
			title = append(title, strconv.Itoa(book.Id), book.Title, book.Author)
			if err := writer.Write(title); err != nil {
				log.Fatalln("error writing record to file", err)
			}
		}
	}
}

func main() {
	request(readFromFile())
	loadTemplates()
	writeInCsv(sqlCon())
	http.HandleFunc("/", welcomeHandler)
	http.HandleFunc("/list", listHandler)
	http.HandleFunc("/form", formHandler)

	err := http.ListenAndServe(":5000", nil)
	if err != nil {
		fmt.Println(err)
	}
}
