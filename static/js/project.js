// project.js

document.addEventListener("DOMContentLoaded", () => {
  // --- API Endpoints ---
  const CARDS_API_URL = "/api/cards/";
  const PROJECTS_API_URL = "/api/projects/";
  const DOCUMENTS_API_URL = "/api/documents/";
  const CLUSTER_API_URL = "/api/projects/cluster";

  // --- DOM 요소 ---
  const projectNameEl = document.getElementById("project-name");
  const cardGridEl = document.getElementById("card-grid");
  const addDocumentBtn = document.getElementById("add-document-btn");
  const clusterBtn = document.getElementById("cluster-btn");

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
  let projectDesc = "";
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
      projectDesc = project.projectdesc || "";
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

  // 카드 목록 렌더링 (메인 콘텐츠) - 카테고리별 그룹화
  function renderCards() {
    cardGridEl.innerHTML = ""; // 기존 내용 비우기

    if (projectDesc) {
      const descEl = document.createElement("div");
      descEl.className = "project-description";
      descEl.textContent = projectDesc;
      cardGridEl.appendChild(descEl);
    }

    // 1. 카드를 카테고리별로 그룹화
    const groupedCards = cards.reduce((acc, card) => {
      const category = card.category || "미분류";
      if (!acc[category]) {
        acc[category] = [];
      }
      acc[category].push(card);
      return acc;
    }, {});

    // 2. 카테고리 순서 정렬 ('미분류'를 맨 뒤로)
    const sortedCategories = Object.keys(groupedCards).sort((a, b) => {
      if (a === "미분류") return 1;
      if (b === "미분류") return -1;
      return a.localeCompare(b);
    });

    // 3. 각 카테고리별로 섹션 렌더링
    sortedCategories.forEach((category) => {
      const categorySection = document.createElement("div");
      categorySection.className = "category-section";

      const categoryTitle = document.createElement("h2");
      categoryTitle.className = "category-title";
      categoryTitle.textContent = category;
      categorySection.appendChild(categoryTitle);

      const innerGrid = document.createElement("div");
      innerGrid.className = "card-grid-inner";
      categorySection.appendChild(innerGrid);

      // 해당 카테고리의 카드들 렌더링
      groupedCards[category].forEach((card) => {
        const cardEl = createCardElement(card);
        innerGrid.appendChild(cardEl);
      });

      cardGridEl.appendChild(categorySection);
    });

    // 새 카드 추가 버튼은 항상 최상위 그리드에 추가
    const addCardEl = createCardElement(null, true);
    cardGridEl.appendChild(addCardEl);
  }

  // 카드 DOM 요소를 생성하는 헬퍼 함수
  function createCardElement(card, isAddButton = false) {
    if (isAddButton) {
      const addCardEl = document.createElement("div");
      addCardEl.className = "card card-add";
      addCardEl.innerHTML = `
        <div class="card-icon-add">+</div>
        <div class="card-name">새 카드 추가</div>
      `;
      addCardEl.addEventListener("click", handleCreateCard);
      return addCardEl;
    }

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

    cardEl.addEventListener("click", (e) => {
      if (e.target.classList.contains("card-delete-btn")) return;
      openCardModal(card);
    });

    const deleteBtn = cardEl.querySelector(".card-delete-btn");
    deleteBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      handleDeleteCard(card.id);
    });

    return cardEl;
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
    listContainer.style.display = "flex";
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
      <iframe class="document-content-iframe" frameborder="0"></iframe>
    `;
    detailContainer.style.display = "flex";

    // iframe에 컨텐츠 삽입
    const iframe = detailContainer.querySelector('.document-content-iframe');
    const iframeDoc = iframe.contentDocument || iframe.contentWindow.document;
    iframeDoc.open();
    iframeDoc.write(`
      <!DOCTYPE html>
      <html>
      <head>
        <meta charset="UTF-8">
        <style>
          body {
            margin: 0;
            padding: 1.5rem;
            font-family: "Pretendard", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            line-height: 1.7;
            color: #8c7b70;
          }
          h1, h2, h3, h4, h5, h6 {
            color: #4d423a;
            margin-top: 1.5rem;
            margin-bottom: 0.5rem;
          }
          h2 {
            font-size: 1.8rem;
            margin-top: 0;
          }
          p {
            margin-bottom: 1rem;
          }
        </style>
      </head>
      <body>
        ${doc.content}
      </body>
      </html>
    `);
    iframeDoc.close();

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
      deleteBtn.textContent = "-";
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

  async function handleClusterCards() {
    if (!confirm("카드를 자동으로 분류할까요? 기존 카테고리 정보는 사라집니다.")) return;

    clusterBtn.textContent = "분류 중...";
    clusterBtn.disabled = true;

    try {
      const response = await fetch(`${CLUSTER_API_URL}?project_id=${projectId}`, {
        method: "POST",
        credentials: "include",
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`클러스터링 실패: ${errorText}`);
      }

      // 성공 후 데이터 다시 로드 및 렌더링
      await fetchCards(projectId);
      renderCards();

    } catch (error) {
      console.error(error);
      alert(error.message);
    } finally {
      clusterBtn.textContent = "카드 클러스터링";
      clusterBtn.disabled = false;
    }
  }

  async function handleCreateCard() {
    const text = prompt("새 카드의 내용을 입력하세요.");
    if (!text || text.trim() === "") return;

    const addCardEl = document.querySelector(".card-add");
    if (!addCardEl) return;

    const originalContent = addCardEl.innerHTML;
    addCardEl.style.pointerEvents = "none"; // 클릭 방지
    addCardEl.innerHTML = `<div class="card-name">AI 태그 생성 중...</div>`;

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
      cards.push(newCard); // 배열에 추가
      renderCards(); // 성공 시, 전체를 다시 렌더링하여 버튼을 자동으로 복구합니다.

    } catch (error) {
      console.error(error);
      alert(error.message);
      // 실패 시, 버튼을 수동으로 복구합니다.
      if (addCardEl) {
        addCardEl.innerHTML = originalContent;
        addCardEl.style.pointerEvents = "auto";
      }
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

    const originalButtonText = addDocumentBtn.textContent;
    addDocumentBtn.disabled = true;
    addDocumentBtn.textContent = "AI 초안 생성 중...";

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
      // 새 문서가 생성되면 바로 상세 뷰를 보여줍니다.
      renderDocumentList(); // 목록을 다시 렌더링하여 새 문서를 반영
      showDetailView(newDocument.id);

    } catch (error) {
      console.error(error);
      alert(error.message);
    } finally {
      // 성공하든 실패하든 버튼 상태를 원상 복구
      addDocumentBtn.disabled = false;
      addDocumentBtn.textContent = originalButtonText;
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
    clusterBtn.addEventListener("click", handleClusterCards);

    await loadData();
    renderCards();
    showListView();

    addDocumentBtn.addEventListener("click", handleAddDocument);
  }

  init();
});