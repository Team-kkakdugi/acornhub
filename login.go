package main 

import (
	"os"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"github.com/golang-jwt/jwt/v5"
)

// GitHub API 응답(JSON)을 파싱하기 위한 구조체
type GitHubUser struct {
	ID       int64  `json:"id"`    // GitHub의 고유 ID
	Username string `json:"login"` // GitHub의 사용자 이름
}

type jwtClaims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func createJWT(userID int64) (string, error) {
	expirationTime := time.Now().Add(8 * time.Hour)

	claims := &jwtClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	jwtKey := []byte(os.Getenv("JWT_SECRET_KEY"))
	if len(jwtKey) == 0 {
		return "", fmt.Errorf("JWT_SECRET_KEY가 설정되지 않았습니다")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// --- 핸들러 함수 ---

func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Expires:  time.Now(),
		HttpOnly: true,
		Path:     "/",
	})

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// 사용자를 GitHub 인증 페이지로 리디렉션
// CSRF 방지를 위해 무작위 state 문자열을 생성하여 쿠키에 저장
func handleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	b := make([]byte, 32)
	rand.Read(b)
	oauthStateString := base64.URLEncoding.EncodeToString(b)

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    oauthStateString,
		Expires:  time.Now().Add(10 * time.Minute),
		HttpOnly: true,
	})

	url := githubOauthConfig.AuthCodeURL(oauthStateString)

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	stateFromURL := r.FormValue("state")

	stateCookie, err := r.Cookie("oauth_state")
	if err != nil {
		fmt.Println("State 쿠키 없음")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	if stateFromURL != stateCookie.Value {
		fmt.Println("Invalid state: URL과 쿠키의 state 불일치")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Expires:  time.Now(),
		HttpOnly: true,
	})

	code := r.FormValue("code")

	token, err := githubOauthConfig.Exchange(context.Background(), code)
	if err != nil {
		fmt.Printf("Code 교환 실패: %s\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	client := githubOauthConfig.Client(context.Background(), token)

	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		fmt.Printf("GitHub 유저 정보 요청 실패: %s\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	defer resp.Body.Close()

	userData, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("유저 정보 응답 읽기 실패: %s\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	var githubUser GitHubUser
	if err = json.Unmarshal(userData, &githubUser); err != nil {
		fmt.Printf("유저 정보 JSON 파싱 실패: %s\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	if db == nil {
		fmt.Println("DB가 초기화되지 않았습니다. (main.go 확인 필요)")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	query := `INSERT OR IGNORE INTO users (id, username) VALUES (?, ?)`
	_, err = db.Exec(query, githubUser.ID, githubUser.Username)
	if err != nil {
		fmt.Printf("DB에 유저 저장 실패: %s\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	fmt.Printf("로그인 성공: %s (ID: %d)\n", githubUser.Username, githubUser.ID)

	tokenString, err := createJWT(githubUser.ID)
	if err != nil {
		fmt.Printf("JWT 생성 실패: %s\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",   // 쿠키 이름 (JWT 저장)
		Value:    tokenString,    // 생성된 JWT 문자열
		Expires:  time.Now().Add(3 * time.Hour), // 쿠키 만료 시간 (토큰과 동일하게)
		HttpOnly: true, // JavaScript에서 접근 불가 (필수 보안)
		Path:     "/",  // 사이트 전체에서 쿠키 사용
	})

	http.Redirect(w, r, "/dashboard.html", http.StatusTemporaryRedirect)
}