// ✅ 1. 백엔드 주소 & 엔드포인트
// ❌ 예전: "https://oli.tailda0655.ts.net/auth/github"  (잘못됨)
// ✅ 수정: 도메인까지만
const API_BASE_URL = "https://oli.tailda0655.ts.net";

// GitHub 로그인 시작 주소
const GITHUB_LOGIN_URL = `${API_BASE_URL}/auth/github`;

// 현재 로그인 사용자 정보 확인
const ME_URL = `${API_BASE_URL}/api/me`;

// 로그인 성공 후 이동할 페이지
const AFTER_LOGIN_URL = "/dashboard.html";

// ✅ 2. 페이지 로드 시: 로그인 상태 확인해서, 이미 로그인되어 있으면 main.html로 보내기
async function checkLoginAndRedirect() {
  console.log("[checkLoginAndRedirect] 시작");

  try {
    const res = await fetch(ME_URL, {
      method: "GET",
      credentials: "include", // 세션 쿠키 같이 보내기 (중요)
    });

    console.log("[checkLoginAndRedirect] status:", res.status);

    if (res.ok) {
      const data = await res.json();
      console.log("[checkLoginAndRedirect] 이미 로그인된 유저:", data);

      // ✅ 로그인된 상태면 main.html로 이동
      window.location.href = AFTER_LOGIN_URL;
    } else {
      // 401, 403 등: 로그인 안 된 상태 → 그냥 index.html에 머무름
      const text = await res.text();
      console.log("[checkLoginAndRedirect] 로그인 안 됨, 응답:", text);
    }
  } catch (err) {
    console.error("[checkLoginAndRedirect] 에러:", err);
  }
}

// ✅ 3. GitHub 로그인 버튼 클릭 시: GitHub 로그인 페이지로 이동
document.addEventListener("DOMContentLoaded", () => {
  const githubLoginBtn = document.getElementById("github-login-btn");

  if (githubLoginBtn) {
    githubLoginBtn.addEventListener("click", (event) => {
      event.preventDefault();
      console.log("[login] GitHub 로그인으로 이동:", GITHUB_LOGIN_URL);
      window.location.href = GITHUB_LOGIN_URL;
    });
  }

  // ✅ 페이지 로드될 때 한번 체크:
  // - GitHub 콜백 후 index.html로 돌아왔을 때
  // - 이미 세션이 있는 상태에서 들어왔을 때
  checkLoginAndRedirect();
});
