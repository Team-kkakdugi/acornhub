// config.go
package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	_ "github.com/mattn/go-sqlite3"
)

var (
	db                *sql.DB
	githubOauthConfig *oauth2.Config
)

func loadConfig() error {
	if err := godotenv.Load(); err != nil {
		fmt.Println("경고: .env 파일을 찾을 수 없습니다.")
	}

	githubOauthConfig = &oauth2.Config{
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		Endpoint:     github.Endpoint,
		RedirectURL:  "https://oli.tailda0655.ts.net/auth/github/callback",
		Scopes:       []string{"read:user"},
	}
	if githubOauthConfig.ClientID == "" {
		fmt.Println("경고: GITHUB_CLIENT_ID가 설정되지 않았습니다.")
	}

	var err error
	db, err = sql.Open("sqlite3", "./main.db")
	if err != nil {
		return fmt.Errorf("DB 열기 실패: %w", err)
	}

	if err = setupDatabase(); err != nil {
		return fmt.Errorf("DB 테이블 생성 실패: %w", err)
	}

	fmt.Println("DB 및 설정 로드 완료.")
	return nil
}

func setupDatabase() error {
	createUsersTableSQL := `
    CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY, 
        username TEXT UNIQUE NOT NULL
    );`
	if _, err := db.Exec(createUsersTableSQL); err != nil {
		return err
	}

	createProjectsTableSQL := `
	CREATE TABLE IF NOT EXISTS projects (
		id INTEGER PRIMARY KEY,
		projectname TEXT UNIQUE NOT NULL,
		projectdesc TEXT,
		user_id INTEGER,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`
	if _, err := db.Exec(createProjectsTableSQL); err != nil {
		return err
	}

	createCardsTableSQL := `
	CREATE TABLE IF NOT EXISTS cards (
		id INTEGER PRIMARY KEY,
		cardtext TEXT,
		cardurl TEXT,
		cardtags TEXT,
		project_id INTEGER,
		user_id INTEGER,
		FOREIGN KEY(project_id) REFERENCES projects(id),
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`
	if _, err := db.Exec(createCardsTableSQL); err != nil {
		return err
	}

	return nil
}