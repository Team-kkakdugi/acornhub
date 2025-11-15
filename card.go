package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Card 구조체 (DB 스키마에 맞춰 정의)
type Card struct {
	CardID    int64  `json:"id"`
	Text      string `json:"cardtext"`
	URL       string `json:"cardurl"`
	Tags      string `json:"cardtags"`
	ProjectID int64  `json:"project_id"`
	UserID    int64  `json:"user_id,omitempty"` // 서버에서 채우므로 클라이언트 요청에는 불필요
}

// handleCards는 카드의 CRUD 작업을 처리하는 핸들러입니다.
// /api/cards 와 /api/cards/{id} 경로의 요청을 처리합니다.
func handleCards(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userContextKey).(int64)
	if !ok {
		http.Error(w, "서버 내부 오류: 유저 ID를 찾을 수 없음", http.StatusInternalServerError)
		return
	}

	// /api/cards/ 또는 /api/cards/{id} 에서 id 부분을 추출
	path := strings.TrimPrefix(r.URL.Path, "/api/cards")
	idFromPath := ""
	if strings.HasPrefix(path, "/") {
		idFromPath = strings.TrimPrefix(path, "/")
	}

	switch r.Method {
	case "POST":
		createCard(w, r, userID)
	case "GET":
		// id가 있으면 특정 카드 조회, 없으면 프로젝트의 카드 목록 조회
		if idFromPath != "" {
			getCard(w, r, userID, idFromPath)
		} else {
			getCardsByProject(w, r, userID)
		}
	case "PUT":
		updateCard(w, r, userID, idFromPath)
	case "DELETE":
		deleteCard(w, r, userID, idFromPath)
	default:
		http.Error(w, "지원하지 않는 메소드", http.StatusMethodNotAllowed)
	}
}

func createCard(w http.ResponseWriter, r *http.Request, userID int64) {
	var card Card
	if err := json.NewDecoder(r.Body).Decode(&card); err != nil {
		http.Error(w, "잘못된 JSON 형식", http.StatusBadRequest)
		return
	}
	card.UserID = userID

	// 사용자가 해당 프로젝트의 소유자인지 확인
	var projectOwnerID int64
	err := db.QueryRow("SELECT user_id FROM projects WHERE id = ?", card.ProjectID).Scan(&projectOwnerID)
	if err != nil || projectOwnerID != userID {
		http.Error(w, "프로젝트를 찾을 수 없거나 권한이 없습니다", http.StatusForbidden)
		return
	}

	// 태그가 비어있을 경우, AI 서버를 호출하여 자동 생성
	if card.Tags == "" && card.Text != "" {
		// AI 서버에 보낼 요청 본문 생성
		reqBody := map[string]string{"content": card.Text}
		reqBytes, err := json.Marshal(reqBody)
		if err == nil {
			// AI 서버에 POST 요청
			resp, err := http.Post("http://127.0.0.1:8000/tags/generate", "application/json", bytes.NewBuffer(reqBytes))
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var tagResp struct {
						Tags []string `json:"tags"`
					}
					if err := json.NewDecoder(resp.Body).Decode(&tagResp); err == nil {
						card.Tags = strings.Join(tagResp.Tags, ",")
					}
				}
			}
		}
		// 프로토타입 단계에서는 AI 태그 생성 실패가 카드 생성 자체를 막지 않도록 함
	}

	result, err := db.Exec(
		"INSERT INTO cards (cardtext, cardurl, cardtags, project_id, user_id) VALUES (?, ?, ?, ?, ?)",
		card.Text, card.URL, card.Tags, card.ProjectID, card.UserID,
	)
	if err != nil {
		http.Error(w, "카드 생성 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	cardID, err := result.LastInsertId()
	if err != nil {
		http.Error(w, "카드 ID 가져오기 실패", http.StatusInternalServerError)
		return
	}
	card.CardID = cardID

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(card)
}

func getCardsByProject(w http.ResponseWriter, r *http.Request, userID int64) {
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

	// 사용자가 해당 프로젝트의 소유자인지 확인
	var projectOwnerID int64
	err = db.QueryRow("SELECT user_id FROM projects WHERE id = ?", projectID).Scan(&projectOwnerID)
	if err != nil || projectOwnerID != userID {
		http.Error(w, "프로젝트에 대한 권한이 없습니다", http.StatusForbidden)
		return
	}

	rows, err := db.Query("SELECT id, cardtext, cardurl, cardtags, project_id, user_id FROM cards WHERE project_id = ? AND user_id = ?", projectID, userID)
	if err != nil {
		http.Error(w, "카드 목록 조회 실패", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		var c Card
		if err := rows.Scan(&c.CardID, &c.Text, &c.URL, &c.Tags, &c.ProjectID, &c.UserID); err != nil {
			http.Error(w, "DB 스캔 실패", http.StatusInternalServerError)
			return
		}
		cards = append(cards, c)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cards)
}

func getCard(w http.ResponseWriter, r *http.Request, userID int64, idFromPath string) {
	cardID, err := strconv.ParseInt(idFromPath, 10, 64)
	if err != nil {
		http.Error(w, "잘못된 id 경로", http.StatusBadRequest)
		return
	}

	var card Card
	err = db.QueryRow("SELECT id, cardtext, cardurl, cardtags, project_id, user_id FROM cards WHERE id = ? AND user_id = ?", cardID, userID).Scan(&card.CardID, &card.Text, &card.URL, &card.Tags, &card.ProjectID, &card.UserID)
	if err != nil {
		http.Error(w, "카드를 찾을 수 없거나 권한이 없습니다", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(card)
}

func updateCard(w http.ResponseWriter, r *http.Request, userID int64, idFromPath string) {
	if idFromPath == "" {
		http.Error(w, "경로에 id가 필요합니다", http.StatusBadRequest)
		return
	}
	cardID, err := strconv.ParseInt(idFromPath, 10, 64)
	if err != nil {
		http.Error(w, "잘못된 id 경로", http.StatusBadRequest)
		return
	}

	var card Card
	if err := json.NewDecoder(r.Body).Decode(&card); err != nil {
		http.Error(w, "잘못된 JSON 형식", http.StatusBadRequest)
		return
	}

	result, err := db.Exec(
		"UPDATE cards SET cardtext = ?, cardurl = ?, cardtags = ? WHERE id = ? AND user_id = ?",
		card.Text, card.URL, card.Tags, cardID, userID,
	)
	if err != nil {
		http.Error(w, "카드 수정 실패", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "수정된 행 수 가져오기 실패", http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		http.Error(w, "카드를 찾을 수 없거나 권한이 없습니다", http.StatusNotFound)
		return
	}

	card.CardID = cardID
	card.UserID = userID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(card)
}

func deleteCard(w http.ResponseWriter, r *http.Request, userID int64, idFromPath string) {
	if idFromPath == "" {
		http.Error(w, "경로에 id가 필요합니다", http.StatusBadRequest)
		return
	}
	cardID, err := strconv.ParseInt(idFromPath, 10, 64)
	if err != nil {
		http.Error(w, "잘못된 id 경로", http.StatusBadRequest)
		return
	}

	result, err := db.Exec("DELETE FROM cards WHERE id = ? AND user_id = ?", cardID, userID)
	if err != nil {
		http.Error(w, "카드 삭제 실패", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "삭제된 행 수 가져오기 실패", http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		http.Error(w, "카드를 찾을 수 없거나 권한이 없습니다", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
