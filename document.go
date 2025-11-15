package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// AgentInvokeRequest는 AI 서버 /agent/invoke 엔드포인트에 보낼 요청 본문입니다.
type AgentInvokeRequest struct {
	Topic         string         `json:"topic"`
	AllTags       []string       `json:"all_tags"`
	AllCategories []CategoryInfo `json:"all_categories"`
	AllCards      []CardForAI    `json:"all_cards"`
}

// CardForAI는 AI 서버에 카드 정보를 전달하기 위한 구조체입니다.
type CardForAI struct {
	ID      int64  `json:"id"`
	Content string `json:"content"`
}

// CategoryInfo는 AI 서버에 카테고리 정보를 전달하기 위한 구조체입니다.
type CategoryInfo struct {
	CategoryName string  `json:"category_name"`
	CardIDs      []int64 `json:"card_ids"`
}

// AgentInvokeResponse는 AI 서버로부터 받을 응답 본문입니다.
type AgentInvokeResponse struct {
	Report string `json:"report"`
}

type Document struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	ProjectID int64     `json:"project_id"`
	UserID    int64     `json:"user_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

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
		createDocumentWithAI(w, r, userID)
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

func getAllCardsForProject(projectID int64, userID int64) ([]CardForAI, error) {
	query := `SELECT id, cardtext FROM cards WHERE project_id = ? AND user_id = ?`
	rows, err := db.Query(query, projectID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []CardForAI
	for rows.Next() {
		var c CardForAI
		if err := rows.Scan(&c.ID, &c.Content); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, nil
}

func getAllTagsForProject(projectID int64, userID int64) ([]string, error) {
	query := `SELECT cardtags FROM cards WHERE project_id = ? AND user_id = ?`
	rows, err := db.Query(query, projectID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tagSet := make(map[string]struct{})
	for rows.Next() {
		var tagsStr string
		if err := rows.Scan(&tagsStr); err != nil {
			return nil, err
		}
		tags := strings.Split(tagsStr, ",")
		for _, tag := range tags {
			trimmedTag := strings.TrimSpace(tag)
			if trimmedTag != "" {
				tagSet[trimmedTag] = struct{}{}
			}
		}
	}

	uniqueTags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		uniqueTags = append(uniqueTags, tag)
	}

	return uniqueTags, nil
}

func getAllCategoriesForProject(projectID int64, userID int64) ([]CategoryInfo, error) {
	query := `SELECT category, GROUP_CONCAT(id) FROM cards WHERE project_id = ? AND user_id = ? AND category IS NOT NULL AND category != '' GROUP BY category`
	rows, err := db.Query(query, projectID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []CategoryInfo
	for rows.Next() {
		var ci CategoryInfo
		var cardIDsStr string
		if err := rows.Scan(&ci.CategoryName, &cardIDsStr); err != nil {
			return nil, err
		}

		ids := strings.Split(cardIDsStr, ",")
		for _, idStr := range ids {
			id, _ := strconv.ParseInt(idStr, 10, 64)
			ci.CardIDs = append(ci.CardIDs, id)
		}
		categories = append(categories, ci)
	}
	return categories, nil
}

func createDocumentWithAI(w http.ResponseWriter, r *http.Request, userID int64) {
	var doc Document
	if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
		http.Error(w, "잘못된 JSON 형식", http.StatusBadRequest)
		return
	}
	doc.UserID = userID

	var projectOwnerID int64
	err := db.QueryRow("SELECT user_id FROM projects WHERE id = ?", doc.ProjectID).Scan(&projectOwnerID)
	if err != nil || projectOwnerID != userID {
		http.Error(w, "프로젝트를 찾을 수 없거나 권한이 없습니다", http.StatusForbidden)
		return
	}

	allCards, err := getAllCardsForProject(doc.ProjectID, userID)
	if err != nil {
		http.Error(w, "카드 정보 조회 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}
	allTags, err := getAllTagsForProject(doc.ProjectID, userID)
	if err != nil {
		http.Error(w, "태그 정보 조회 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}
	allCategories, err := getAllCategoriesForProject(doc.ProjectID, userID)
	if err != nil {
		http.Error(w, "카테고리 정보 조회 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	aiRequestData := AgentInvokeRequest{
		Topic:         doc.Title,
		AllTags:       allTags,
		AllCategories: allCategories,
		AllCards:      allCards,
	}
	jsonData, err := json.Marshal(aiRequestData)
	if err != nil {
		http.Error(w, "AI 요청 데이터 생성 실패", http.StatusInternalServerError)
		return
	}

	aiServerURL := "http://127.0.0.1:8000/agent/invoke"
	resp, err := http.Post(aiServerURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, "AI 서버 호출 실패: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorMsg := fmt.Sprintf("AI 서버 오류 (상태 코드: %d): %s", resp.StatusCode, string(bodyBytes))
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	var aiResponse AgentInvokeResponse
	if err := json.NewDecoder(resp.Body).Decode(&aiResponse); err != nil {
		http.Error(w, "AI 서버 응답 파싱 실패", http.StatusInternalServerError)
		return
	}
	doc.Content = aiResponse.Report

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

	var projectOwnerID int64
	err = db.QueryRow("SELECT user_id FROM projects WHERE id = ?", projectID).Scan(&projectOwnerID)
	if err != nil || projectOwnerID != userID {
		http.Error(w, "프로젝트에 대한 권한이 없습니다", http.StatusForbidden)
		return
	}

	rows, err := db.Query("SELECT id, title, content, project_id, user_id, created_at, updated_at FROM documents WHERE project_id = ? AND user_id = ? ORDER BY created_at DESC", projectID, userID)
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
		if err == sql.ErrNoRows {
			http.Error(w, "문서를 찾을 수 없거나 권한이 없습니다", http.StatusNotFound)
		} else {
			http.Error(w, "문서 조회 실패", http.StatusInternalServerError)
		}
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