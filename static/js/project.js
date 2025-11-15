// project.js

document.addEventListener("DOMContentLoaded", () => {
  // --- API Endpoints ---
  const CARDS_API_URL = "/api/cards/";
  const PROJECTS_API_URL = "/api/projects/";
  const DOCUMENTS_API_URL = "/api/documents/";

  // --- DOM 요소 ---
  const projectNameEl = document.getElementById("project-name");
  const cardGridEl = document.getElementById("card-grid");
  const addDocumentBtn = document.getElementById("add-document-btn");

  // 사이드바 뷰 컨테이너
  const listContainer = document.getElementById("document-list-container");
  const detailContainer = document.getElementById("document-detail-container");
  const documentListEl = document.getElementById("document-list");

  // 모달 요소
  const modalContainer = document.getElementById("card-modal");
  const modalCardText = document.getElementById("modal-card-text");
  const modalCloseBtn = modalContainer.querySelector(".modal-close-btn");
  const modalOverlay = modalContainer.querySelector(".modal-overlay");

  // --- 상태 관리 ---
  let projectId = null;
  let documents = [];
  let cards = [];

  /* ---------------- 데이터 로드 및 렌더링 ---------------- */

  function getProjectIdFromUrl() {
    const params = new URLSearchParams(window.location.search);
    return params.get("id");
  }

  async function fetchProjectDetails(id) {
    try {
      const response = await fetch(`${PROJECTS_API_URL}${id}`, {
        credentials: "include",
      });
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const project = await response.json();
      projectNameEl.textContent = project.projectname || "Unnamed Project";
    } catch (error) {
      console.error("Error fetching project details:", error);
      projectNameEl.textContent = "프로젝트를 찾을 수 없습니다.";
    }
  }

  async function fetchCards(id) {
    try {
      const response = await fetch(`${CARDS_API_URL}?project_id=${id}`, {
        credentials: "include",
      });
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data = await response.json();
      cards = data || [];
    } catch (error) {
      console.error("Error fetching cards:", error);
      cards = [];
    }
  }

  async function fetchDocuments(id) {
    try {
      const response = await fetch(`${DOCUMENTS_API_URL}?project_id=${id}`, {
        credentials: "include",
      });
      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }
      const data = await response.json();
      documents = data || [];
    } catch (error) {
      console.error("Error fetching documents:", error);
      documents = [];
    }
  }

  async function loadData() {
    await fetchProjectDetails(projectId);
    await fetchCards(projectId);
    await fetchDocuments(projectId);
  }

  // 카드 목록 렌더링 (메인 콘텐츠)
  function renderCards() {
    cardGridEl.innerHTML = "";

    const addCardEl = document.createElement("div");
    addCardEl.className = "card card-add";
    addCardEl.innerHTML = `
      <div class="card-icon-add">+</div>
      <div class="card-name">새 카드 추가</div>
    `;
    addCardEl.addEventListener("click", handleCreateCard);
    cardGridEl.appendChild(addCardEl);

    cards.forEach((card) => {
      const cardEl = document.createElement("div");
      cardEl.className = "card";
      cardEl.dataset.cardId = card.id;

      const tags = card.cardtags
        ? card.cardtags.split(",").map((tag) => tag.trim())
        : [];
      const tagsHtml = tags
        .map((tag) => `<span class="card-tag">#${tag}</span>`)
        .join(" ");

      cardEl.innerHTML = `
        <div>
          <div class="card-tags">${tagsHtml}</div>
          <p class="card-text">${card.cardtext}</p>
        </div>
        <button class="card-delete-btn">&times;</button>
      `;

      // 카드 클릭 시 모달 열기
      cardEl.addEventListener("click", (e) => {
        if (e.target.classList.contains("card-delete-btn")) {
          return; // 삭제 버튼 클릭 시 모달 열기 방지
        }
        openCardModal(card);
      });

      // 삭제 버튼 이벤트 리스너
      const deleteBtn = cardEl.querySelector(".card-delete-btn");
      deleteBtn.addEventListener("click", (e) => {
        e.stopPropagation(); // 이벤트 버블링 방지
        handleDeleteCard(card.id);
      });

      cardGridEl.appendChild(cardEl);
    });
  }

  /* ---------------- 모달 관련 함수 ---------------- */

  function openCardModal(card) {
    modalCardText.textContent = card.cardtext;
    modalContainer.style.display = "flex";
  }

  function closeCardModal() {
    modalContainer.style.display = "none";
    modalCardText.textContent = "";
  }

  /* ---------------- 사이드바 (문서) 뷰 전환 로직 ---------------- */

  function showListView() {
    detailContainer.style.display = "none";
    listContainer.style.display = "block";
    renderDocumentList();
  }

  function showDetailView(docId) {
    const doc = documents.find((d) => d.id === docId);
    if (!doc) {
      console.error(`Document with id ${docId} not found.`);
      showListView();
      return;
    }

    listContainer.style.display = "none";
    detailContainer.innerHTML = `
      <div class="document-detail-header">
        <button class="btn-text" id="back-to-list-btn">&larr; 목록으로 돌아가기</button>
      </div>
      <div class="document-content">
        ${doc.content}
      </div>
    `;
    detailContainer.style.display = "block";

    document
      .getElementById("back-to-list-btn")
      .addEventListener("click", showListView);
  }

  function renderDocumentList() {
    documentListEl.innerHTML = "";
    documents.forEach((doc) => {
      const docEl = document.createElement("li");
      docEl.className = "document-list-item";
      docEl.dataset.id = doc.id;

      const titleEl = document.createElement("span");
      titleEl.className = "document-title";
      titleEl.textContent = doc.title;

      const deleteBtn = document.createElement("button");
      deleteBtn.className = "btn-delete-doc";
      deleteBtn.textContent = "–";
      deleteBtn.title = "문서 삭제";

      titleEl.addEventListener("click", () => {
        showDetailView(doc.id);
      });

      deleteBtn.addEventListener("click", (e) => {
        e.stopPropagation();
        handleDeleteDocument(doc.id, doc.title);
      });

      docEl.appendChild(titleEl);
      docEl.appendChild(deleteBtn);
      documentListEl.appendChild(docEl);
    });
  }

  /* ---------------- 이벤트 핸들러 ---------------- */

  async function handleCreateCard() {
    const text = prompt("새 카드의 내용을 입력하세요.");
    if (!text || text.trim() === "") return;

    try {
      const response = await fetch(CARDS_API_URL, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          cardtext: text.trim(),
          project_id: parseInt(projectId, 10),
        }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`카드 생성 실패: ${errorText}`);
      }
      
      const newCard = await response.json();
      cards.unshift(newCard);
      renderCards();

    } catch (error) {
      console.error(error);
      alert(error.message);
    }
  }

  async function handleDeleteCard(cardId) {
    if (!confirm("이 카드를 정말 삭제하시겠습니까?")) return;

    try {
      const response = await fetch(`${CARDS_API_URL}${cardId}`, {
        method: "DELETE",
        credentials: "include",
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`카드 삭제 실패: ${errorText}`);
      }

      // 로컬 상태 업데이트 및 리렌더링
      cards = cards.filter((card) => card.id !== cardId);
      renderCards();

    } catch (error) {
      console.error(error);
      alert(error.message);
    }
  }

  async function handleAddDocument() {
    const title = prompt("새 문서의 제목을 입력하세요.");
    if (!title || title.trim() === "") return;

    try {
      const response = await fetch(DOCUMENTS_API_URL, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          title: title.trim(),
          project_id: parseInt(projectId, 10),
        }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`문서 생성 실패: ${errorText}`);
      }
      
      const newDocument = await response.json();
      documents.unshift(newDocument);
      showDetailView(newDocument.id);

    } catch (error) {
      console.error(error);
      alert(error.message);
    }
  }

  async function handleDeleteDocument(docId, docTitle) {
    if (!confirm(`'${docTitle}' 문서를 정말 삭제하시겠습니까?`)) return;

    try {
      const response = await fetch(`${DOCUMENTS_API_URL}${docId}`, {
        method: "DELETE",
        credentials: "include",
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`문서 삭제 실패: ${errorText}`);
      }

      documents = documents.filter((d) => d.id !== docId);
      showListView();

    } catch (error) {
      console.error(error);
      alert(error.message);
    }
  }

  /* ---------------- 초기화 ---------------- */

  async function init() {
    projectId = getProjectIdFromUrl();
    if (!projectId) {
      projectNameEl.textContent = "잘못된 접근입니다.";
      alert("프로젝트 ID가 없습니다. 대시보드로 돌아갑니다.");
      window.location.href = "/dashboard.html";
      return;
    }

    modalCloseBtn.addEventListener("click", closeCardModal);
    modalOverlay.addEventListener("click", closeCardModal);

    await loadData();
    renderCards();
    showListView();

    addDocumentBtn.addEventListener("click", handleAddDocument);
  }

  init();
});
