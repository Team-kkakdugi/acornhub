// main.go
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func handleMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userContextKey).(int64)
	if !ok {
		http.Error(w, "서버 내부 오류: 유저 ID를 찾을 수 없음", http.StatusInternalServerError)
		return
	}

	if r.Method == "GET" {
		var userName string

		query := `SELECT username FROM users WHERE id = ?;`
		err := db.QueryRow(query, userID).Scan(&userName)
		if err != nil {
			if err == sql.ErrNoRows {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprint(w, "유저 정보가 없습니다.")
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, "DB 조회 실패")
			}
			return
		}

		respData := struct {
			Username string `json:"user_name"`
		}{
			Username: userName,
		}

		jsonData, err := json.Marshal(respData)
		if err != nil {
			http.Error(w, "JSON 인코딩 실패", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonData)
	}
}

func main() {
	if err := loadConfig(); err != nil {
		log.Fatalf("설정 로드 실패: %v", err)
	}
	defer db.Close()

	// API 및 인증 핸들러 등록
	http.HandleFunc("/auth/logout", handleLogout)
	http.HandleFunc("/auth/github", handleGitHubLogin)
	http.HandleFunc("/auth/github/callback", handleGitHubCallback)
	http.HandleFunc("/api/me", authMiddleware(handleMe))
	http.HandleFunc("/api/projects/", authMiddleware(handleProjects))
	http.HandleFunc("/api/cards/", authMiddleware(handleCards))
	http.HandleFunc("/api/documents/", authMiddleware(handleDocuments))

	// 정적 파일 서버 설정
	// 위에서 등록된 API 경로 외의 모든 요청은 static 디렉토리의 파일을 제공합니다.
	// 예를 들어, "/" 요청은 "static/index.html"을, "/css/index.css" 요청은 "static/css/index.css" 파일을 반환합니다.
	http.Handle("/", http.FileServer(http.Dir("./static")))

	fmt.Println("서버가 8080 포트에서 실행 중입니다...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("서버 실행 오류: %v", err)
	}
}
