package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	"os"
	"regexp"
	"strings"
	"time"
)

func main() {

	dbURL := os.Getenv("TURSO_DATABASE_NAME")
	dbAuthToken := os.Getenv("TURSO_DATABASE_AUTH_TOKEN")
	dbUrl := fmt.Sprintf("%s?authToken=%s", dbURL, dbAuthToken)

	db, err := sql.Open("libsql", dbUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open db %s: %s", dbUrl, err)
		os.Exit(1)
	}
	defer db.Close()
	onehourBackTime := time.Now().Add(time.Hour * -1).Format("2006-01-02 15:04:05")

	query := fmt.Sprintf("SELECT * FROM posts WHERE created_at > '%s';", onehourBackTime)

	rows, err := db.Query(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to query db %s: %s", dbUrl, err)
		os.Exit(1)
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var title string
		var slug string
		var body string
		var created string
		var updated string
		var metadata string
		var authorId int64
		err := rows.Scan(&id, &title, &slug, &body, &metadata, &created, &updated, &authorId)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(id, title, slug, body, created, updated, metadata, authorId)
		writePostFile(title, slug, body, metadata, created, updated)
	}
}

func writePostFile(title, slug, body, metadataStr, created, updated string) {
	metadata := make(map[string]interface{})
	fmt.Println(metadataStr)
	err := json.Unmarshal([]byte(metadataStr), &metadata)
	fmt.Println(metadata)
	if err != nil {
		panic(err)
	}
	var postDir string
	var baseDir string = "posts/"
	if val, ok := metadata["post_dir"]; ok {
		postDir = val.(string)
	} else {
		postDir = "posts"
	}
	postDir = baseDir + postDir
	//create folder if not exists
	if _, err := os.Stat(postDir); os.IsNotExist(err) {
		os.Mkdir(postDir, 0777)
	}
	_, ok := metadata["slug"]
	if !ok {
		slug = Slugify(title)
	}
	filePath := fmt.Sprintf("%s/%s.md", postDir, slug)
	fileContent := fmt.Sprintf("%s\n\n%s", metadataStr, body)
	os.WriteFile(filePath, []byte(fileContent), 0660)

}

func Slugify(input string) string {
	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		panic(err)
	}
	processedString := reg.ReplaceAllString(input, " ")
	processedString = strings.TrimSpace(processedString)
	slug := strings.ReplaceAll(processedString, " ", "-")
	slug = strings.ToLower(slug)
	return slug
}
