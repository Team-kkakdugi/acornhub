// document.go
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Document 구조체 (DB 스키마 기반)
type Document struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	ProjectID int64     `json:"project_id"`
	UserID    int64     `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// handleDocuments는 문서의 CRUD 작업을 처리합니다.
func handleDocuments(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userContextKey).(int64)
	if !ok {
		http.Error(w, "서버 내부 오류: 유저 ID를 찾을 수 없음", http.StatusInternalServerError)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/documents/")
	idFromPath := path

	switch r.Method {
	case "POST":
		createDocument(w, r, userID)
	case "GET":
		if idFromPath != "" {
			getDocument(w, r, userID, idFromPath)
		} else {
			getDocumentsByProject(w, r, userID)
		}
	case "PUT":
		updateDocument(w, r, userID, idFromPath)
	case "DELETE":
		deleteDocument(w, r, userID, idFromPath)
	default:
		http.Error(w, "지원하지 않는 메소드", http.StatusMethodNotAllowed)
	}
}

// generateDummyContent는 새 문서의 기본 내용을 생성합니다.
func generateDummyContent(title string) string {
	return fmt.Sprintf("<h2>%s</h2><p>새로 생성된 문서입니다. 내용을 입력하세요.</p>", title)
}

func createDocument(w http.ResponseWriter, r *http.Request, userID int64) {
	var doc Document
	if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
		http.Error(w, "잘못된 JSON 형식", http.StatusBadRequest)
		return
	}
	doc.UserID = userID
	doc.Content = generateDummyContent(doc.Title) // 자동 생성된 내용 할당

	// 사용자가 해당 프로젝트의 소유자인지 확인
	var projectOwnerID int64
	err := db.QueryRow("SELECT user_id FROM projects WHERE id = ?", doc.ProjectID).Scan(&projectOwnerID)
	if err != nil || projectOwnerID != userID {
		http.Error(w, "프로젝트를 찾을 수 없거나 권한이 없습니다", http.StatusForbidden)
		return
	}

	query := "INSERT INTO documents (title, content, project_id, user_id) VALUES (?, ?, ?, ?)"
	result, err := db.Exec(query, doc.Title, doc.Content, doc.ProjectID, doc.UserID)
	if err != nil {
		http.Error(w, "문서 생성 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	docID, err := result.LastInsertId()
	if err != nil {
		http.Error(w, "문서 ID 가져오기 실패", http.StatusInternalServerError)
		return
	}
	doc.ID = docID

	// 생성된 전체 문서를 다시 조회하여 반환 (타임스탬프 등 포함)
	err = db.QueryRow("SELECT created_at, updated_at FROM documents WHERE id = ?", docID).Scan(&doc.CreatedAt, &doc.UpdatedAt)
	if err != nil {
		http.Error(w, "생성된 문서 정보 조회 실패", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(doc)
}

func getDocumentsByProject(w http.ResponseWriter, r *http.Request, userID int64) {
	projectIDStr := r.URL.Query().Get("project_id")
	if projectIDStr == "" {
		http.Error(w, "project_id 쿼리 파라미터가 필요합니다", http.StatusBadRequest)
		return
	}
	projectID, err := strconv.ParseInt(projectIDStr, 10, 64)
	if err != nil {
		http.Error(w, "잘못된 project_id", http.StatusBadRequest)
		return
	}

	// 프로젝트 소유권 확인
	var projectOwnerID int64
	err = db.QueryRow("SELECT user_id FROM projects WHERE id = ?", projectID).Scan(&projectOwnerID)
	if err != nil || projectOwnerID != userID {
		http.Error(w, "프로젝트에 대한 권한이 없습니다", http.StatusForbidden)
		return
	}

	rows, err := db.Query("SELECT id, title, content, project_id, user_id, created_at, updated_at FROM documents WHERE project_id = ? AND user_id = ?", projectID, userID)
	if err != nil {
		http.Error(w, "문서 목록 조회 실패", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var documents []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.Title, &d.Content, &d.ProjectID, &d.UserID, &d.CreatedAt, &d.UpdatedAt); err != nil {
			http.Error(w, "DB 스캔 실패", http.StatusInternalServerError)
			return
		}
		documents = append(documents, d)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(documents)
}

func getDocument(w http.ResponseWriter, r *http.Request, userID int64, idFromPath string) {
	docID, err := strconv.ParseInt(idFromPath, 10, 64)
	if err != nil {
		http.Error(w, "잘못된 id 경로", http.StatusBadRequest)
		return
	}

	var doc Document
	query := "SELECT id, title, content, project_id, user_id, created_at, updated_at FROM documents WHERE id = ? AND user_id = ?"
	err = db.QueryRow(query, docID, userID).Scan(&doc.ID, &doc.Title, &doc.Content, &doc.ProjectID, &doc.UserID, &doc.CreatedAt, &doc.UpdatedAt)
	if err != nil {
		http.Error(w, "문서를 찾을 수 없거나 권한이 없습니다", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(doc)
}

func updateDocument(w http.ResponseWriter, r *http.Request, userID int64, idFromPath string) {
	docID, err := strconv.ParseInt(idFromPath, 10, 64)
	if err != nil {
		http.Error(w, "잘못된 id 경로", http.StatusBadRequest)
		return
	}

	var doc Document
	if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
		http.Error(w, "잘못된 JSON 형식", http.StatusBadRequest)
		return
	}

	query := "UPDATE documents SET title = ?, content = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?"
	result, err := db.Exec(query, doc.Title, doc.Content, docID, userID)
	if err != nil {
		http.Error(w, "문서 수정 실패", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "문서를 찾을 수 없거나 권한이 없습니다", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func deleteDocument(w http.ResponseWriter, r *http.Request, userID int64, idFromPath string) {
	docID, err := strconv.ParseInt(idFromPath, 10, 64)
	if err != nil {
		http.Error(w, "잘못된 id 경로", http.StatusBadRequest)
		return
	}

	result, err := db.Exec("DELETE FROM documents WHERE id = ? AND user_id = ?", docID, userID)
	if err != nil {
		http.Error(w, "문서 삭제 실패", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "문서를 찾을 수 없거나 권한이 없습니다", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
