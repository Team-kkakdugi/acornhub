package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type Project struct {
	Projectid int64  `json:"projectid"`
	Name      string `json:"projectname"`
	Desc      string `json:"projectdesc"`
	Userid    int64  `json:"user_id"`
}

func handleProjects(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(userContextKey).(int64)
	if !ok {
		http.Error(w, "서버 내부 오류: 유저 ID를 찾을 수 없음", http.StatusInternalServerError)
		return
	}

	// support both /api/projects and /api/projects/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/projects")
	idFromPath := ""
	if strings.HasPrefix(path, "/") {
		idFromPath = strings.TrimPrefix(path, "/")
	}

	if r.Method == "POST" {
		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "잘못된 요청: Content-Type이 application/json이 아님", http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "요청 본문 읽기 실패", http.StatusInternalServerError)
			return
		}

		var project Project
		if err := json.Unmarshal(body, &project); err != nil {
			http.Error(w, "잘못된 JSON 형식", http.StatusBadRequest)
			return
		}

		project.Userid = userID

		result, err := db.Exec(
			"INSERT INTO projects (projectname, projectdesc, user_id) VALUES (?, ?, ?)",
			project.Name, project.Desc, project.Userid,
		)
		if err != nil {
			http.Error(w, "프로젝트 생성 실패", http.StatusInternalServerError)
			return
		}

		projectID, err := result.LastInsertId()
		if err != nil {
			http.Error(w, "프로젝트 ID 가져오기 실패", http.StatusInternalServerError)
			return
		}
		project.Projectid = projectID

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(project); err != nil {
			http.Error(w, "응답 인코딩 실패", http.StatusInternalServerError)
			return
		}

	} else if r.Method == "GET" {
		searchQuery := r.URL.Query().Get("q")

		var rows *sql.Rows
		var err error

		if searchQuery != "" {
			// 검색어가 있는 경우: 공백 제거 및 소문자 변환 후 LIKE 검색
			searchTerm := strings.ToLower(strings.ReplaceAll(searchQuery, " ", ""))
			query := `
				SELECT id, projectname, projectdesc, user_id 
				FROM projects 
				WHERE user_id = ? AND LOWER(REPLACE(projectname, ' ', '')) LIKE ?
			`
			rows, err = db.Query(query, userID, "%"+searchTerm+"%")
		} else {
			// 검색어가 없는 경우: 모든 프로젝트 조회
			query := "SELECT id, projectname, projectdesc, user_id FROM projects WHERE user_id = ?"
			rows, err = db.Query(query, userID)
		}

		if err != nil {
			http.Error(w, "프로젝트 목록 조회 실패", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var list []Project
		for rows.Next() {
			var p Project
			if err := rows.Scan(&p.Projectid, &p.Name, &p.Desc, &p.Userid); err != nil {
				http.Error(w, "DB 스캔 실패", http.StatusInternalServerError)
				return
			}
			list = append(list, p)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(list); err != nil {
			http.Error(w, "응답 인코딩 실패", http.StatusInternalServerError)
			return
		}

	} else if r.Method == "DELETE" {
		// support DELETE /api/projects/{id} or DELETE /api/projects with JSON body
		var targetID int64
		if idFromPath != "" {
			id64, err := strconv.ParseInt(idFromPath, 10, 64)
			if err != nil {
				http.Error(w, "잘못된 id 경로", http.StatusBadRequest)
				return
			}
			targetID = id64
		} else {
			// fallback to JSON body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "요청 본문 읽기 실패", http.StatusInternalServerError)
				return
			}
			var reqData struct {
				Projectid int64 `json:"projectid"`
			}
			if err := json.Unmarshal(body, &reqData); err != nil {
				http.Error(w, "잘못된 JSON 형식", http.StatusBadRequest)
				return
			}
			targetID = reqData.Projectid
		}

		result, err := db.Exec(
			"DELETE FROM projects WHERE id = ? AND user_id = ?",
			targetID, userID,
		)
		if err != nil {
			http.Error(w, "프로젝트 삭제 실패", http.StatusInternalServerError)
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			http.Error(w, "삭제된 행 수 가져오기 실패", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			http.Error(w, "프로젝트를 찾을 수 없거나 권한이 없습니다", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)

	} else if r.Method == "PUT" {
		// require id in path for PUT
		if idFromPath == "" {
			http.Error(w, "경로에 id가 필요합니다", http.StatusBadRequest)
			return
		}
		id64, err := strconv.ParseInt(idFromPath, 10, 64)
		if err != nil {
			http.Error(w, "잘못된 id 경로", http.StatusBadRequest)
			return
		}

		if r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "잘못된 요청: Content-Type이 application/json이 아님", http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "요청 본문 읽기 실패", http.StatusInternalServerError)
			return
		}

		var project Project
		if err := json.Unmarshal(body, &project); err != nil {
			http.Error(w, "잘못된 JSON 형식", http.StatusBadRequest)
			return
		}

		// only allow owner to update
		result, err := db.Exec(
			"UPDATE projects SET projectname = ?, projectdesc = ? WHERE id = ? AND user_id = ?",
			project.Name, project.Desc, id64, userID,
		)
		if err != nil {
			http.Error(w, "프로젝트 수정 실패", http.StatusInternalServerError)
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			http.Error(w, "수정된 행 수 가져오기 실패", http.StatusInternalServerError)
			return
		}
		if rowsAffected == 0 {
			http.Error(w, "프로젝트를 찾을 수 없거나 권한이 없습니다", http.StatusNotFound)
			return
		}

		// respond with updated project
		project.Projectid = id64
		project.Userid = userID
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(project); err != nil {
			http.Error(w, "응답 인코딩 실패", http.StatusInternalServerError)
			return
		}
	}
}
