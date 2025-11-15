// dashboard.js

// 실제 백엔드 주소
const API_BASE_URL = "https://oli.tailda0655.ts.net";
const PROJECT_API_URL = `${API_BASE_URL}/api/project`;
const LOGOUT_URL = `${API_BASE_URL}/auth/logout`;

const logoutBtn = document.getElementById("logout-btn");
const addFolderCard = document.getElementById("add-folder-card");
const folderList = document.getElementById("folder-list");

// 프론트에서 들고 있을 프로젝트 목록
let projects = [];

/* ---------------- 로그아웃 ---------------- */

if (logoutBtn) {
  logoutBtn.addEventListener("click", async () => {
    try {
      const res = await fetch(LOGOUT_URL, {
        method: "POST",
        credentials: "include",
      });

      console.log("[LOGOUT] status:", res.status);
      if (!res.ok) {
        const text = await res.text();
        console.error("[LOGOUT] error body:", text);
      }
    } catch (e) {
      console.error("로그아웃 에러(무시 가능):", e);
    } finally {
      window.location.href = "/index.html"; // 로그인 페이지 경로에 맞게
    }
  });
}

/* ---------------- 프로젝트 목록 불러오기 ---------------- */

async function fetchProjects() {
  try {
    const res = await fetch(PROJECT_API_URL, {
      method: "GET",
      credentials: "include",
    });

    console.log("[GET /api/project] status:", res.status);

    if (res.status === 401) {
      console.warn("[GET /api/project] 401 -> 로그인 필요");
      window.location.href = "/index.html";
      return;
    }

    if (!res.ok) {
      const text = await res.text();
      console.error("[GET /api/project] error body:", text);
      alert("프로젝트 목록 불러오기 실패\n" + text);
      return;
    }

    const data = await res.json();
    console.log("[GET /api/project] response json:", data);

    // [{ id, name }, ...] 형태라고 가정
    projects = Array.isArray(data) ? data : [];
    renderProjects();
  } catch (err) {
    console.error(err);
    alert("프로젝트 목록을 불러오는 중 오류가 발생했어요.");
  }
}

/* ---------------- 이름 중복 체크 ---------------- */

function isDuplicateFolderName(name) {
  const normalized = name.trim().toLowerCase();
  return projects.some(
    (p) => (p.name || "").trim().toLowerCase() === normalized
  );
}

/* ---------------- 폴더 카드 생성 ---------------- */

function createFolderCard(project) {
  const card = document.createElement("div");
  card.className = "folder-card";
  card.dataset.id = project.id;

  const img = document.createElement("img");
  img.src = "assets/folder.png";
  img.alt = "폴더";
  img.className = "folder-image";

  const nameEl = document.createElement("div");
  nameEl.className = "folder-name";
  nameEl.textContent = project.name;

  const deleteBtn = document.createElement("button");
  deleteBtn.type = "button";
  deleteBtn.className = "folder-delete-button";
  deleteBtn.textContent = "×";

  // 삭제
  deleteBtn.addEventListener("click", (event) => {
    event.stopPropagation();
    handleDeleteProject(project);
  });

  // 카드 클릭 시, 나중에 상세 페이지 이동 등 넣고 싶으면 여기에서
  card.addEventListener("click", () => {
    console.log("클릭된 프로젝트:", project);
    // 예: window.location.href = `/project.html?id=${project.id}`;
  });

  card.appendChild(img);
  card.appendChild(nameEl);
  card.appendChild(deleteBtn);

  return card;
}

/* ---------------- 프로젝트 목록 렌더링 ---------------- */

function renderProjects() {
  folderList.innerHTML = "";
  projects.forEach((project) => {
    const card = createFolderCard(project);
    folderList.appendChild(card);
  });
}

/* ---------------- 프로젝트 생성 ---------------- */

async function handleCreateProject() {
  let name = prompt("새 폴더 이름을 입력하세요.");
  if (name === null) return; // 취소
  name = name.trim();
  if (!name) return;

  if (isDuplicateFolderName(name)) {
    alert("같은 이름의 폴더가 이미 있어요. 다른 이름을 입력해 주세요.");
    return;
  }

  try {
    console.log("[POST /api/project] request body:", { name });

    const res = await fetch(PROJECT_API_URL, {
      method: "POST",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
      },
      // 🔥 여기 body 필드 이름이 서버 요구사항이랑 다르면 400이 뜰 거야
      body: JSON.stringify({ name }),
    });

    console.log("[POST /api/project] status:", res.status);

    if (res.status === 401) {
      console.warn("[POST /api/project] 401 -> 로그인 필요");
      window.location.href = "/index.html";
      return;
    }

    if (!res.ok) {
      const text = await res.text();
      console.error("[POST /api/project] error body:", text);
      alert("프로젝트 생성 실패\n" + text);
      return;
    }

    const created = await res.json();
    console.log("[POST /api/project] response json:", created);

    // 배열 맨 앞에 추가
    projects.unshift(created);
    renderProjects();
  } catch (err) {
    console.error(err);
    alert("프로젝트를 생성하는 중 오류가 발생했어요.");
  }
}

/* ---------------- 프로젝트 삭제 ---------------- */

async function handleDeleteProject(project) {
  const ok = confirm(`'${project.name}' 폴더를 삭제할까요?`);
  if (!ok) return;

  try {
    console.log("[DELETE /api/project/:id] id:", project.id);

    const res = await fetch(`${PROJECT_API_URL}/${project.id}`, {
      method: "DELETE",
      credentials: "include",
    });

    console.log("[DELETE /api/project/:id] status:", res.status);

    if (res.status === 401) {
      console.warn("[DELETE /api/project/:id] 401 -> 로그인 필요");
      window.location.href = "/index.html";
      return;
    }

    if (!res.ok) {
      const text = await res.text();
      console.error("[DELETE /api/project/:id] error body:", text);
      alert("프로젝트 삭제 실패\n" + text);
      return;
    }

    // 로컬 배열에서 제거
    projects = projects.filter((p) => p.id !== project.id);
    renderProjects();
  } catch (err) {
    console.error(err);
    alert("프로젝트를 삭제하는 중 오류가 발생했어요.");
  }
}

/* ---------------- 초기화 ---------------- */

document.addEventListener("DOMContentLoaded", () => {
  // 처음 들어왔을 때 백엔드에서 목록 가져오기
  fetchProjects();

  if (addFolderCard) {
    addFolderCard.addEventListener("click", handleCreateProject);
  }
});
