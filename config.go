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
		category TEXT,
		project_id INTEGER,
		user_id INTEGER,
		FOREIGN KEY(project_id) REFERENCES projects(id),
		FOREIGN KEY(user_id) REFERENCES users(id)
	);`
	if _, err := db.Exec(createCardsTableSQL); err != nil {
		return err
	}

	// documents 테이블에 대한 스키마 마이그레이션 (기존 코드와 동일한 패턴)
	if err := migrateDocumentsTable(); err != nil {
		return err
	}

	// cards 테이블에 category 컬럼이 없는 구버전 스키마를 위한 마이그레이션
	rows, err := db.Query("PRAGMA table_info(cards)")
	if err != nil {
		return err
	}
	defer rows.Close()

	var categoryColumnExists bool
	for rows.Next() {
		var cid int
		var name string
		var type_ string
		var notnull bool
		var dflt_value interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &type_, &notnull, &dflt_value, &pk); err != nil {
			return err
		}
		if name == "category" {
			categoryColumnExists = true
			break
		}
	}

	if !categoryColumnExists {
		fmt.Println("기존 cards 테이블에 'category' 컬럼이 없어 추가합니다...")
		_, err := db.Exec("ALTER TABLE cards ADD COLUMN category TEXT")
		if err != nil {
			return err
		}
	}

	createDocumentsTableSQL := `
	CREATE TABLE IF NOT EXISTS documents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		content TEXT,
		project_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (project_id) REFERENCES projects (id),
		FOREIGN KEY (user_id) REFERENCES users (id)
	);`
	if _, err := db.Exec(createDocumentsTableSQL); err != nil {
		return err
	}

	return nil
}

// migrateDocumentsTable는 documents 테이블의 스키마를 최신 상태로 유지합니다.
func migrateDocumentsTable() error {
	// 예시: 만약 나중에 documents 테이블에 'status' 컬럼이 추가된다면,
	// 아래와 같은 마이그레이션 로직을 추가할 수 있습니다.
	// 현재는 특별한 변경사항이 없으므로 비워둡니다.
	return nil
}