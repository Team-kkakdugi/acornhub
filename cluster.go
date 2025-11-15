package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Python AI 서버 /cards/cluster 엔드포인트에 보내는 요청 형식
type ClusterAIRequest struct {
	Cards []ClusterCard `json:"cards"`
}
type ClusterCard struct {
	ID      int64  `json:"id"`
	Content string `json:"content"`
}

// Python AI 서버 /cards/cluster 엔드포인트에서 받는 응답 형식
type ClusterAIResponse struct {
	Clusters []ClusterInfo `json:"clusters"`
}
type ClusterInfo struct {
	CategoryName string  `json:"category_name"`
	CardIDs      []int64 `json:"card_ids"`
}

// POST /api/projects/cluster?project_id={id}
func handleCluster(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST 메소드만 지원합니다.", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value(userContextKey).(int64)
	if !ok {
		http.Error(w, "서버 내부 오류: 유저 ID를 찾을 수 없음", http.StatusInternalServerError)
		return
	}

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

	rows, err := db.Query("SELECT id, cardtext FROM cards WHERE project_id = ? AND user_id = ?", projectID, userID)
	if err != nil {
		http.Error(w, "카드 목록 조회 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var aiRequest ClusterAIRequest
	for rows.Next() {
		var card ClusterCard
		if err := rows.Scan(&card.ID, &card.Content); err != nil {
			http.Error(w, "DB 스캔 실패: "+err.Error(), http.StatusInternalServerError)
			return
		}
		aiRequest.Cards = append(aiRequest.Cards, card)
	}

	if len(aiRequest.Cards) == 0 {
		http.Error(w, "클러스터링할 카드가 없습니다.", http.StatusBadRequest)
		return
	}

	reqBytes, err := json.Marshal(aiRequest)
	if err != nil {
		http.Error(w, "AI 요청 JSON 직렬화 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp, err := http.Post("http://127.0.0.1:8000/cards/cluster", "application/json", bytes.NewBuffer(reqBytes))
	if err != nil {
		http.Error(w, "AI 서버 호출 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, fmt.Sprintf("AI 서버가 오류를 반환했습니다 (상태 코드: %d)", resp.StatusCode), http.StatusInternalServerError)
		return
	}

	var aiResponse ClusterAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&aiResponse); err != nil {
		http.Error(w, "AI 응답 JSON 디코딩 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 트랜잭션을 사용하여 여러 업데이트를 원자적으로 처리합니다.
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "DB 트랜잭잭션 시작 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// 먼저 모든 카드를 '미분류'로 초기화
	_, err = tx.Exec("UPDATE cards SET category = '미분류' WHERE project_id = ?", projectID)
	if err != nil {
		tx.Rollback()
		http.Error(w, "카테고리 초기화 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	for _, cluster := range aiResponse.Clusters {
		if len(cluster.CardIDs) == 0 {
			continue
		}
		args := make([]interface{}, len(cluster.CardIDs)+2)
		args[0] = cluster.CategoryName
		args[1] = projectID
		for i, id := range cluster.CardIDs {
			args[i+2] = id
		}

		query := "UPDATE cards SET category = ? WHERE project_id = ? AND id IN (?" + strings.Repeat(",?", len(cluster.CardIDs)-1) + ")"

		stmt, err := tx.Prepare(query)
		if err != nil {
			tx.Rollback()
			http.Error(w, "DB 구문 준비 실패: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		if _, err := stmt.Exec(args...); err != nil {
			tx.Rollback()
			http.Error(w, "카테고리 업데이트 실패: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "DB 트랜잭션 커밋 실패: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "카드 클러스터링 및 업데이트가 성공적으로 완료되었습니다.")
}
