# Acorn Hub 백엔드 시스템

Acorn Hub는 자기주도형 자료 조사 및 학습을 돕는 AI 기반 웹 애플리케이션입니다. 사용자는 관련 자료(카드)를 수집하고, AI 기능을 통해 자료를 자동으로 분류, 태그하고, 최종적으로 구조화된 문서(리포트)를 생성할 수 있습니다.

이 문서는 Acorn Hub의 서버 아키텍처, 사용된 기술 스택, 그리고 핵심 AI 기능들의 데이터 파이프라인에 대해 상세히 설명합니다.

## 1. 기술 스택

Acorn Hub는 Go로 작성된 메인 백엔드 서버와 Python으로 작성된 AI 서버, 두 개의 독립적인 서버로 구성되어 있습니다.

### 1.1. 메인 백엔드 (Go)

-   **언어**: Go 1.22
-   **웹 프레임워크**: `net/http`
-   **데이터베이스**: SQLite 3 (`github.com/mattn/go-sqlite3`)
-   **인증**:
    -   OAuth 2.0 (`golang.org/x/oauth2`) - GitHub 로그인
    -   JWT (`github.com/golang-jwt/jwt/v5`) - 세션 관리
-   **설정 관리**: `.env` 파일 (`github.com/joho/godotenv`)
-   **역할**:
    -   사용자 인증 및 세션 관리
    -   정적 파일(HTML, CSS, JS) 서빙
    -   프로젝트, 카드, 문서 데이터에 대한 CRUD API 제공
    -   AI 서버와의 통신 중계

### 1.2. AI 서버 (Python)

-   **언어**: Python 3.11+
-   **웹 프레임워크**: FastAPI
-   **서버**: Uvicorn
-   **LLM 및 AI 라이브러리**:
    -   **LLM API**: Google Gemini, Anthropic Claude
    -   **임베딩**: Google Gemini (`gemini-embedding-001`)
    -   **군집화**: Scikit-learn (`KMeans`)
    -   **키워드/태그 추출**:
        -   `konlpy` (Okt) - 한국어 명사, 구문 분석
        -   `KeyBERT` - 키워드 및 키프레이즈 추출
        -   `transformers` (Hugging Face) - NER 모델 실행
-   **역할**:
    -   Go 백엔드의 요청을 받아 복잡한 AI 연산 수행
    -   AI 기능 API 제공 (카드 태그 생성, 카드 군집화, 문서 초안 생성)

### 1.3. 프런트엔드

-   **기술**: Vanilla JavaScript (ES6+), HTML5, CSS3
-   **API 통신**: `fetch` API
-   **역할**:
    -   사용자 인터페이스 제공 (로그인, 대시보드, 프로젝트 뷰)
    -   Go 백엔드 API와 상호작용하여 데이터 표시 및 사용자 입력 처리

## 2. 서버 아키텍처


### 2.1. 전체 흐름

1.  **사용자 접속**: 사용자는 웹 브라우저를 통해 Acorn Hub에 접속합니다. Go 백엔드는 `index.html`과 관련 정적 파일들을 서빙합니다.
2.  **인증**:
    -   사용자가 'GitHub로 로그인' 버튼을 클릭하면, Go 백엔드의 `/auth/github` 엔드포인트가 호출됩니다.
    -   서버는 사용자를 GitHub 인증 페이지로 리디렉션합니다.
    -   인증 후 GitHub는 설정된 콜백 URL (`/auth/github/callback`)로 사용자를 리디렉션합니다.
    -   Go 백엔드는 콜백을 받아 GitHub API로 사용자 정보를 조회하고, `users` 테이블에 사용자를 저장(또는 업데이트)합니다.
    -   서버는 사용자 ID를 담은 JWT(JSON Web Token)를 생성하여 `HttpOnly` 속성의 `auth_token` 쿠키에 저장하고, 사용자를 `/dashboard.html`로 리디렉션합니다.
3.  **API 요청**:
    -   대시보드 및 프로젝트 페이지의 모든 동적 데이터 요청(프로젝트 목록, 카드 생성 등)은 JavaScript의 `fetch`를 통해 Go 백엔드의 `/api/*` 엔드포인트로 전송됩니다.
    -   모든 `/api/*` 요청은 `authMiddleware`를 통과합니다. 이 미들웨어는 요청 쿠키에서 JWT를 검증하고, 유효한 경우 요청 컨텍스트에 사용자 ID를 주입합니다.
    -   각 API 핸들러는 컨텍스트의 사용자 ID를 사용하여 해당 사용자에게 권한이 있는 데이터만 처리(CRUD)합니다.
4.  **AI 기능 요청**:
    -   사용자가 AI 기능(예: 카드 클러스터링)을 트리거하면, 프런트엔드는 Go 백엔드의 특정 API(예: `/api/projects/cluster`)를 호출합니다.
    -   Go 백엔드는 DB에서 AI 연산에 필요한 데이터(예: 프로젝트의 모든 카드 텍스트)를 조회합니다.
    -   Go 백엔드는 이 데이터를 가지고 Python AI 서버의 해당 엔드포인트(예: `/cards/cluster`)에 HTTP 요청을 보냅니다.
    -   AI 서버는 연산을 수행하고 결과를 Go 백엔드에 반환합니다.
    -   Go 백엔드는 AI 서버의 결과를 받아 DB를 업데이트하고, 최종 결과를 프런트엔드에 응답합니다.

### 2.2. 데이터베이스 스키마

-   `users`: 사용자 정보 (GitHub ID, 사용자명)
-   `projects`: 프로젝트 정보 (이름, 설명, 소유자 user_id)
-   `cards`: 자료 카드 정보 (텍스트, URL, 태그, 카테고리, 속한 project_id, 소유자 user_id)
-   `documents`: AI가 생성한 문서 정보 (제목, HTML 콘텐츠, 속한 project_id, 소유자 user_id)

## 3. AI 기능 및 데이터 파이프라인

Acorn Hub는 3가지 핵심 AI 기능을 제공합니다.

### 3.1. 기능 1: AI 태그 자동 생성

-   **목표**: 사용자가 새 카드를 생성할 때, 카드 내용(텍스트)을 분석하여 관련 태그를 자동으로 추천하고 저장합니다.
-   **트리거**: 프런트엔드에서 `cardtext`는 있지만 `cardtags`가 비어있는 상태로 카드 생성 API (`POST /api/cards/`)를 호출합니다.

#### 데이터 파이프라인:

1.  **[Frontend → Go] 요청**: `project.js`에서 사용자가 입력한 카드 텍스트를 JSON 형식으로 Go 백엔드의 `createCard` 핸들러에 전송합니다.
2.  **[Go → AI Server] 중계**: `createCard` 핸들러는 `cardtags` 필드가 비어있는 것을 확인하고, 카드 텍스트를 담아 Python AI 서버의 `/tags/generate` 엔드포인트에 HTTP POST 요청을 보냅니다.
3.  **[AI Server] 태그 추출**:
    a.  `/tags/generate` 엔드포인트는 다양한 알고리즘을 병렬로 사용하여 후보 태그를 추출합니다.
        -   **형태소 분석**: `konlpy.Okt`로 텍스트에서 명사만 추출하여 빈도수 기반 상위 태그 선정.
        -   **키프레이즈 추출**: `KeyBERT` 모델로 1-2단어 조합의 핵심 구문 추출.
        -   **개체명 인식(NER)**: `soddokayo/klue-roberta-large-klue-ner` 모델로 텍스트 내의 고유 개체(기관, 인물, 제품 등)를 추출.
    b.  추출된 모든 후보 태그를 중복 제거하여 하나의 리스트로 만듭니다.
    c.  이 후보 리스트를 프롬프트에 담아 **Google Gemini** 모델에 전달하며, "이 후보들 중 핵심적인 태그 5개만 골라 다듬어달라"고 요청합니다.
    d.  Gemini가 정제한 최종 태그 리스트(예: `["AI", "데이터 파이프라인", "Go"]`)를 JSON으로 응답합니다.
4.  **[AI Server → Go] 응답**: Go 백엔드는 AI 서버로부터 최종 태그 리스트를 받습니다.
5.  **[Go] DB 저장**: `createCard` 핸들러는 받은 태그들을 쉼표로 구분된 단일 문자열로 합치고, 다른 카드 정보와 함께 `cards` 테이블의 `cardtags` 필드에 저장합니다.
6.  **[Go → Frontend] 최종 응답**: 새로 생성된 카드 정보(AI 태그 포함)를 프런트엔드에 반환하여 UI에 즉시 표시되도록 합니다.

### 3.2. 기능 2: 카드 자동 군집화 (클러스터링)

-   **목표**: 프로젝트에 수집된 여러 카드들을 내용의 유사도에 따라 자동으로 그룹화하고, 각 그룹에 적절한 카테고리 이름을 붙여줍니다.
-   **트리거**: 사용자가 프로젝트 뷰에서 '카드 클러스터링' 버튼을 클릭하여 `POST /api/projects/cluster` API를 호출합니다.

#### 데이터 파이프라인:

1.  **[Frontend → Go] 요청**: `project.js`에서 현재 `project_id`를 쿼리 파라미터로 하여 Go 백엔드의 `handleCluster` 핸들러에 요청을 보냅니다.
2.  **[Go] 데이터 준비**: `handleCluster` 핸들러는 DB에서 해당 `project_id`를 가진 모든 카드의 `id`와 `cardtext`를 조회합니다.
3.  **[Go → AI Server] 중계**: 조회된 카드 목록(`[{"id": 1, "content": "..."}, ...]`)을 JSON 본문에 담아 Python AI 서버의 `/cards/cluster` 엔드포인트에 HTTP POST 요청을 보냅니다.
4.  **[AI Server] 군집화 및 네이밍**:
    a.  **임베딩**: `/cards/cluster` 엔드포인트는 **Google Gemini 임베딩 모델**을 호출하여 각 카드의 `content`를 고차원 벡터로 변환합니다.
    b.  **K-Means 군집화**: `scikit-learn`의 `KMeans` 알고리즘을 사용하여 의미적으로 유사한 카드 벡터들을 K개의 그룹(클러스터)으로 묶습니다. (K는 카드 개수에 따라 동적으로 결정)
    c.  **LLM 기반 네이밍**: 각 클러스터에 속한 카드들의 텍스트를 모아 프롬프트를 구성하고, **Google Gemini** 모델에 "이 텍스트들의 공통 주제를 가장 잘 나타내는 2~3 단어의 카테고리 이름을 지어달라"고 요청합니다. 이 과정을 각 클러스터마다 반복합니다.
    d.  최종적으로 생성된 클러스터 정보(예: `[{"category_name": "AI 기술 동향", "card_ids": [1, 5, 8]}, ...]`)를 JSON으로 응답합니다.
5.  **[AI Server → Go] 응답**: Go 백엔드는 AI 서버로부터 카테고리 이름과 해당 카테고리에 속한 카드 ID 목록을 받습니다.
6.  **[Go] DB 업데이트**: `handleCluster` 핸들러는 DB 트랜잭션을 시작하고, 응답받은 정보를 바탕으로 `cards` 테이블의 `category` 필드를 일괄 업데이트합니다.
7.  **[Go → Frontend] 최종 응답**: 성공 메시지를 프런트엔드에 반환합니다. 프런트엔드는 이 응답을 받고 카드 목록을 새로고침하여 카테고리별로 재정렬된 UI를 보여줍니다.

### 3.3. 기능 3: AI 문서 초안 자동 생성

-   **목표**: 프로젝트에 수집된 모든 카드, 태그, 카테고리 정보를 종합적으로 활용하여 특정 주제에 대한 구조화된 문서(리포트)의 초안을 자동으로 작성합니다.
-   **트리거**: 사용자가 프로젝트 뷰에서 '새 문서 +' 버튼을 클릭하고 문서 제목을 입력하여 `POST /api/documents/` API를 호출합니다.

#### 데이터 파이프라인:

1.  **[Frontend → Go] 요청**: `project.js`에서 사용자가 입력한 문서 제목(`title`)과 현재 `project_id`를 JSON으로 Go 백엔드의 `createDocumentWithAI` 핸들러에 전송합니다.
2.  **[Go] 컨텍스트 데이터 수집**: `createDocumentWithAI` 핸들러는 `project_id`를 이용해 DB에서 다음 정보들을 모두 조회합니다.
    -   프로젝트 내 모든 카드의 내용 (`id`, `cardtext`)
    -   프로젝트 내 모든 태그 (중복 제거)
    -   프로젝트 내 모든 카테고리 정보 (카테고리 이름과 소속 카드 ID 목록)
3.  **[Go → AI Server] 중계**: 수집된 모든 정보와 사용자가 입력한 문서 `title`을 JSON 본문에 담아 Python AI 서버의 `/agent/invoke` 엔드포인트에 HTTP POST 요청을 보냅니다.
4.  **[AI Server] AI 에이전트 실행**:
    a.  `/agent/invoke` 엔드포인트는 **Anthropic Claude 4.5 Sonnet** 모델을 기반으로 하는 AI 에이전트를 실행합니다.
    b.  **Tool 정의**: 에이전트는 `search_cards(categories)`라는 함수(Tool)를 사용할 수 있도록 정의됩니다. 이 함수는 특정 카테고리에 속한 카드들의 내용을 검색하는 역할을 합니다.
    c.  **프롬프트 구성**: 에이전트에게 최종 목표("입력된 주제에 대한 보고서를 HTML 형식으로 작성하라"), 사용 가능한 정보(전체 태그, 전체 카테고리 목록), 그리고 작업 절차(관련 카테고리 선택 → `search_cards`로 정보 수집 → 보고서 작성)를 포함한 상세한 프롬프트를 전달합니다.
    d.  **자율적 추론 및 Tool 사용**:
        -   에이전트는 주제와 가장 관련 있는 카테고리들을 스스로 판단하여 `search_cards` 함수를 호출합니다.
        -   AI 서버는 이 함수 호출을 인터셉트하여, Go로부터 전달받은 카드 데이터에서 해당 내용을 찾아 에이전트에게 반환합니다.
        -   에이전트는 검색된 카드 내용을 바탕으로 '개요', '서론', '본론', '결론'의 구조를 갖춘 보고서 초안을 **HTML 형식**으로 생성합니다.
    e.  최종 생성된 HTML 문자열을 JSON으로 응답합니다.
5.  **[AI Server → Go] 응답**: Go 백엔드는 AI 에이전트가 생성한 HTML 형식의 보고서 초안을 받습니다.
6.  **[Go] DB 저장**: `createDocumentWithAI` 핸들러는 받은 HTML 콘텐츠를 `documents` 테이블의 `content` 필드에 저장합니다.
7.  **[Go → Frontend] 최종 응답**: 새로 생성된 문서의 전체 정보(ID, 제목, AI 생성 콘텐츠 등)를 프런트엔드에 반환합니다. 프런트엔드는 이 정보를 받아 문서 목록을 업데이트하고, 방금 생성된 문서의 상세 뷰를 즉시 표시합니다.

## 4. 실행 방법

이 프로젝트를 로컬 환경에서 실행하려면 Go와 Python 실행 환경이 필요하며, 두 개의 서버를 동시에 실행해야 합니다.

### 4.1. 사전 준비

1.  **Go 설치**: Go 1.22 이상 버전을 설치합니다.
2.  **Python 설치**: Python 3.11 이상 버전을 설치하고 `pip`와 `venv`를 사용할 수 있도록 설정합니다.
3.  **환경 변수 설정**:
    -   프로젝트 루트 디렉터리에 `.env` 파일을 생성합니다.
    -   아래 내용을 참고하여 파일에 키를 추가합니다. GitHub OAuth App과 각 AI 서비스에서 API 키를 발급받아야 합니다.

    ```env
    # GitHub OAuth Application credentials
    GITHUB_CLIENT_ID=your_github_client_id
    GITHUB_CLIENT_SECRET=your_github_client_secret

    # JWT secret key (any random string)
    JWT_SECRET_KEY=your_super_secret_key

    # AI Service API Keys
    GEMINI_API_KEY=your_google_gemini_api_key
    ANTHROPIC_API_KEY=your_anthropic_claude_api_key
    ```

### 4.2. 서버 실행

두 개의 터미널을 열고 각각 다음 단계를 진행합니다.

#### 터미널 1: Python AI 서버 실행

1.  **가상 환경 생성 및 활성화**:
    ```bash
    python -m venv .venv
    source .venv/bin/activate  # macOS/Linux
    # .\.venv\Scripts\activate  # Windows
    ```

2.  **Python 의존성 설치**:
    ```bash
    pip install -r requirements.txt
    ```

3.  **AI 서버 시작**:
    ```bash
    uvicorn ai_server:app --reload
    ```
    서버가 시작되면 `http://127.0.0.1:8000`에서 실행됩니다.

#### 터미널 2: Go 백엔드 서버 실행

1.  **Go 의존성 설치** (최초 실행 시 또는 `go.mod` 변경 시):
    ```bash
    go mod tidy
    ```

2.  **Go 백엔드 서버 시작**:
    ```bash
    go run .
    ```
    서버가 시작되면 `http://localhost:8080`에서 실행됩니다.

### 4.3. 애플리케이션 접속

-   웹 브라우저를 열고 `http://localhost:8080`으로 접속하여 Acorn Hub를 사용할 수 있습니다.
-   **참고**: GitHub OAuth App 설정에서 "Authorization callback URL"을 `https://oli.tailda0655.ts.net/auth/github/callback`과 같이 실제 배포된 URL(또는 로컬 테스트용 URL)로 정확하게 설정해야 로그인이 정상적으로 동작합니다. 로컬에서만 테스트하는 경우, `http://localhost:8080/auth/github/callback`으로 설정하고 `config.go` 파일의 `RedirectURL`도 동일하게 수정해야 합니다.
