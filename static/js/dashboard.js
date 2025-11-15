// dashboard.js

// 실제 백엔드 주소
const API_BASE_URL = "https://oli.tailda0655.ts.net";
const PROJECT_LIST_URL = `${API_BASE_URL}/api/projects/`;   // 목록/생성 (슬래시 추가)
const PROJECT_DELETE_URL = `${API_BASE_URL}/api/projects/`;  // 삭제 (URL에 ID 추가 필요)
const PROJECT_SEARCH_URL = `${API_BASE_URL}/api/projects/`; // 검색 (슬래시 추가)
const LOGOUT_URL = `${API_BASE_URL}/auth/logout`;
const ME_URL = `${API_BASE_URL}/api/me`;

const logoutBtn = document.getElementById("logout-btn");
const addFolderCard = document.getElementById("add-folder-card");
const folderList = document.getElementById("folder-list");

// 사이드바 요소
const sidebar = document.getElementById("sidebar");
const sidebarToggle = document.getElementById("sidebar-toggle");
const sidebarProjectList = document.getElementById("sidebar-project-list");
const addPageBtn = document.getElementById("add-page-btn");
const userNameLabel = document.getElementById("user-name-label");

// 상단 검색창
const searchInput = document.getElementById("project-search-input");

// 프론트에서 들고 있을 프로젝트 목록
let projects = [];

/* ---------------- 사이드바 토글 ---------------- */

if (sidebarToggle && sidebar) {
  sidebarToggle.addEventListener("click", () => {
    sidebar.classList.toggle("collapsed");
    // 아이콘 변경
    if (sidebar.classList.contains("collapsed")) {
      sidebarToggle.textContent = "›";
    } else {
      sidebarToggle.textContent = "‹";
    }
  });
}

/* ---------------- 유저 이름 불러오기 ---------------- */

async function fetchMeAndSetName() {
  if (!userNameLabel) return;

  try {
    const res = await fetch(ME_URL, {
      method: "GET",
      credentials: "include",
    });

    if (res.status === 401) {
      window.location.href = "/index.html";
      return;
    }

    if (!res.ok) {
      console.error("[GET /api/me] status:", res.status);
      return;
    }

    const data = await res.json();
    console.log("[GET /api/me] response:", data);

    // API 응답: user_name 또는 github_username
    const name = data.user_name || data.github_username || "사용자";
    userNameLabel.textContent = name;
  } catch (err) {
    console.error("[GET /api/me] error:", err);
  }
}

/* ---------------- 로그아웃 ---------------- */

if (logoutBtn) {
  logoutBtn.addEventListener("click", async () => {
    try {
      await fetch(LOGOUT_URL, {
        method: "POST",
        credentials: "include",
      });
    } catch (e) {
      console.error("로그아웃 에러(무시 가능):", e);
    } finally {
      window.location.href = "/index.html";
    }
  });
}

/* ---------------- 프로젝트 목록 불러오기 (전체) ---------------- */

async function fetchProjects() {
  console.log("=== fetchProjects 시작 ===");
  
  try {
    const res = await fetch(PROJECT_LIST_URL, {
      method: "GET",
      credentials: "include",
    });

    console.log("프로젝트 목록 조회 - Status:", res.status);

    if (res.status === 401) {
      console.error("401 Unauthorized - 로그인이 필요합니다.");
      window.location.href = "/index.html";
      return;
    }

    if (!res.ok) {
      const text = await res.text();
      console.error("[GET /api/projects] error:", text);
      projects = [];
      renderProjects();
      renderSidebarProjects();
      return;
    }

    // 응답 텍스트 먼저 확인
    const responseText = await res.text();
    console.log("[GET /api/projects] response text:", responseText);
    
    let data;
    try {
      data = responseText ? JSON.parse(responseText) : null;
    } catch (parseError) {
      console.error("JSON 파싱 에러:", parseError);
      console.error("원본 텍스트:", responseText);
      data = null;
    }
    
    console.log("[GET /api/projects] parsed response:", data);
    console.log("[GET /api/projects] response 타입:", typeof data);
    console.log("[GET /api/projects] 배열인가?", Array.isArray(data));

    // API 응답이 배열로 온다고 가정
    if (Array.isArray(data)) {
      projects = data;
    } else if (data === null || data === undefined) {
      console.warn("응답이 null 또는 undefined입니다. 빈 배열로 초기화합니다.");
      projects = [];
    } else {
      console.warn("응답이 배열이 아닙니다:", data);
      projects = [];
    }
    
    console.log("최종 projects:", projects);
    console.log("프로젝트 개수:", projects.length);
    
    renderProjects();
    renderSidebarProjects();
  } catch (err) {
    console.error("[GET /api/projects] 에러:", err);
    projects = []; // 에러 발생 시 빈 배열로 초기화
    renderProjects();
    renderSidebarProjects();
  }
}

/* ---------------- 프로젝트 검색 ---------------- */

async function searchProjects(keyword) {
  const query = keyword.trim();
  if (!query) {
    fetchProjects();
    return;
  }

  try {
    const url = `${PROJECT_SEARCH_URL}?q=${encodeURIComponent(query)}`;
    const res = await fetch(url, {
      method: "GET",
      credentials: "include",
    });

    if (res.status === 401) {
      window.location.href = "/index.html";
      return;
    }

    if (!res.ok) {
      const text = await res.text();
      console.error("[GET /api/projects(search)] error:", text);
      return;
    }

    const data = await res.json();
    console.log("[GET /api/projects(search)] response:", data);

    projects = Array.isArray(data) ? data : [];
    renderProjects();
    renderSidebarProjects();
  } catch (err) {
    console.error(err);
  }
}

/* ---------------- 이름 중복 체크 ---------------- */

function isDuplicateFolderName(name) {
  console.log("중복 체크 - 입력된 이름:", name);
  console.log("중복 체크 - 현재 projects:", projects);
  console.log("중복 체크 - projects 타입:", typeof projects);
  console.log("중복 체크 - projects가 배열인가?", Array.isArray(projects));
  
  if (!projects || !Array.isArray(projects)) {
    console.warn("projects가 배열이 아닙니다!");
    return false;
  }
  
  const normalized = name.trim().toLowerCase();
  const isDuplicate = projects.some(
    (p) => {
      if (!p) return false;
      const projectName = p.projectname || "";
      return projectName.trim().toLowerCase() === normalized;
    }
  );
  
  console.log("중복 여부:", isDuplicate);
  return isDuplicate;
}

/* ---------------- 폴더 카드 생성 ---------------- */

function createFolderCard(project) {
  const card = document.createElement("div");
  card.className = "folder-card";
  card.dataset.id = project.projectid;

  // CSS로 만든 폴더 (div)
  const folderDiv = document.createElement("div");
  folderDiv.className = "folder-image";

  // 이름 - 폴더 안에 넣기
  const nameEl = document.createElement("div");
  nameEl.className = "folder-name";
  nameEl.textContent = project.projectname;
  
  // 폴더 안에 이름 추가
  folderDiv.appendChild(nameEl);

  // 삭제 버튼
  const deleteBtn = document.createElement("button");
  deleteBtn.type = "button";
  deleteBtn.className = "folder-delete-button";
  deleteBtn.textContent = "×";

  deleteBtn.addEventListener("click", (event) => {
    event.stopPropagation();
    handleDeleteProject(project);
  });

  // 카드 클릭 시 상세 페이지로 이동
  card.addEventListener("click", () => {
    if (project && project.projectid) {
      window.location.href = `/project.html?id=${project.projectid}`;
    } else {
      console.error("프로젝트 ID를 찾을 수 없습니다.", project);
      alert("프로젝트 정보를 여는 데 실패했습니다.");
    }
  });

  card.appendChild(folderDiv);
  card.appendChild(deleteBtn);

  return card;
}

/* ---------------- 메인 영역 프로젝트 렌더링 ---------------- */

function renderProjects() {
  folderList.innerHTML = "";
  projects.forEach((project) => {
    const card = createFolderCard(project);
    folderList.appendChild(card);
  });
}

/* ---------------- 사이드바 프로젝트 리스트 렌더링 ---------------- */

function renderSidebarProjects() {
  if (!sidebarProjectList) return;

  sidebarProjectList.innerHTML = "";

  projects.forEach((project) => {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "sidebar-project-item";
    btn.textContent = project.projectname;
    btn.addEventListener("click", () => {
      if (project && project.projectid) {
        window.location.href = `/project.html?id=${project.projectid}`;
      }
    });
    sidebarProjectList.appendChild(btn);
  });
}

/* ---------------- 프로젝트 생성 ---------------- */

async function handleCreateProject() {
  console.log("=== handleCreateProject 함수 시작 ===");
  
  let name = prompt("새 폴더 이름을 입력하세요.");
  console.log("입력받은 이름:", name);
  
  if (name === null) {
    console.log("취소됨");
    return;
  }
  
  name = name.trim();
  if (!name) {
    console.log("빈 문자열");
    return;
  }

  if (isDuplicateFolderName(name)) {
    alert("같은 이름의 폴더가 이미 있어요. 다른 이름을 입력해 주세요.");
    return;
  }

  console.log("=== fetch 요청 보내기 시작 ===");
  console.log("URL:", PROJECT_LIST_URL);
  console.log("Body:", JSON.stringify({ projectname: name }));

  try {
    const res = await fetch(PROJECT_LIST_URL, {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({ projectname: name }),
    });

    console.log("=== fetch 응답 받음 ===");
    console.log("Status:", res.status);
    console.log("OK:", res.ok);

    if (res.status === 401) {
      console.log("401 Unauthorized - 로그인 페이지로 이동");
      window.location.href = "/index.html";
      return;
    }

    if (!res.ok) {
      const text = await res.text();
      console.error("[POST /api/projects] error:", text);
      alert("프로젝트 생성 실패\n" + text);
      return;
    }

    let created;
    try {
      created = await res.json();
    } catch (parseError) {
      console.error("[POST /api/projects] JSON 파싱 실패:", parseError);
      alert("프로젝트가 생성되었을 수 있습니다. 목록을 새로고침합니다.");
      await fetchProjects();
      return;
    }

    console.log("[POST /api/projects] 전체 응답:", created);
    console.log("[POST /api/projects] 응답 타입:", typeof created);
    
    if (created) {
      console.log("[POST /api/projects] 응답 키들:", Object.keys(created || {}));
    }

    // 응답이 없거나 비어있거나 projectid가 없는 경우
    if (!created || !created.projectid) {
      console.warn("서버 응답이 없거나 projectid가 없습니다.");
      console.warn("받은 응답:", created);
      console.log("전체 목록을 다시 불러옵니다.");
      await fetchProjects();
      return;
    }

    // 정상적인 경우
    console.log("=== 프로젝트를 배열에 추가하고 렌더링 ===");
    projects.unshift(created);
    renderProjects();
    renderSidebarProjects();
    console.log("=== 완료 ===");
  } catch (err) {
    console.error("[POST /api/projects] 에러:", err);
    alert("프로젝트를 생성하는 중 오류가 발생했어요: " + err.message);
  }
}

/* ---------------- 프로젝트 삭제 ---------------- */

async function handleDeleteProject(project) {
  const ok = confirm(`'${project.projectname}' 폴더를 삭제할까요?`);
  if (!ok) return;

  try {
    // API 명세서: DELETE /api/projects/{id}
    const deleteUrl = `${PROJECT_DELETE_URL}${project.projectid}`;
    console.log("삭제 요청 URL:", deleteUrl);
    
    const res = await fetch(deleteUrl, {
      method: "DELETE",
      credentials: "include",
    });

    console.log("삭제 응답 Status:", res.status);

    if (res.status === 401) {
      window.location.href = "/index.html";
      return;
    }

    if (!res.ok) {
      const text = await res.text();
      console.error("[DELETE /api/projects/{id}] error:", text);
      alert("프로젝트 삭제 실패\n" + text);
      return;
    }

    // 로컬 배열에서 제거
    projects = projects.filter((p) => p.projectid !== project.projectid);
    renderProjects();
    renderSidebarProjects();
    console.log("프로젝트 삭제 완료:", project.projectname);
  } catch (err) {
    console.error(err);
    alert("프로젝트를 삭제하는 중 오류가 발생했어요.");
  }
}

/* ---------------- 초기화 ---------------- */

document.addEventListener("DOMContentLoaded", () => {
  fetchMeAndSetName();
  fetchProjects();

  if (addFolderCard) {
    addFolderCard.addEventListener("click", handleCreateProject);
  }

  if (addPageBtn) {
    addPageBtn.addEventListener("click", handleCreateProject);
  }

  if (searchInput) {
    searchInput.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        searchProjects(searchInput.value);
      }
    });
  }
});