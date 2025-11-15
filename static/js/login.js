const API_BASE_URL = "https://oli.tailda0655.ts.net";
const GITHUB_LOGIN_URL = `${API_BASE_URL}/auth/github`;
const ME_URL = `${API_BASE_URL}/api/me/`;
const AFTER_LOGIN_URL = "/dashboard.html";

async function checkLoginAndRedirect() {
  console.log("[checkLoginAndRedirect] 시작");

  try {
    const res = await fetch(ME_URL, {
      method: "GET",
      credentials: "include",
    });

    console.log("[checkLoginAndRedirect] status:", res.status);

    if (res.ok) {
      const data = await res.json();
      console.log("[checkLoginAndRedirect] 이미 로그인된 유저:", data);

      window.location.href = AFTER_LOGIN_URL;
    } else {
      const text = await res.text();
      console.log("[checkLoginAndRedirect] 로그인 안 됨, 응답:", text);
    }
  } catch (err) {
    console.error("[checkLoginAndRedirect] 에러:", err);
  }
}

document.addEventListener("DOMContentLoaded", () => {
  const githubLoginBtn = document.getElementById("github-login-btn");

  if (githubLoginBtn) {
    githubLoginBtn.addEventListener("click", (event) => {
      event.preventDefault();
      console.log("[login] GitHub 로그인으로 이동:", GITHUB_LOGIN_URL);
      window.location.href = GITHUB_LOGIN_URL;
    });
  }

  checkLoginAndRedirect();
});