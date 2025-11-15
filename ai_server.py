# ai_server.py
# FastAPI를 사용한 AI 모델 서버

# --- 1. 라이브러리 임포트 ---
import os
import re
import json
import warnings
from collections import Counter

import uvicorn
from fastapi import FastAPI
from pydantic import BaseModel, Field
from typing import List, Dict, Any

import numpy as np
from sklearn.cluster import KMeans
import google.genai as genai
from dotenv import load_dotenv

from konlpy.tag import Okt
from keybert import KeyBERT
from transformers import pipeline

# --- 2. 기본 설정 및 경고 무시 ---
warnings.filterwarnings("ignore", category=FutureWarning)
load_dotenv() # .env 파일에서 환경 변수 로드

# --- 3. FastAPI 앱 및 데이터 모델 정의 ---
app = FastAPI()

# API 요청/응답을 위한 Pydantic 모델
class TagGenerationRequest(BaseModel):
    content: str

class TagGenerationResponse(BaseModel):
    tags: List[str]

class Card(BaseModel):
    id: int
    content: str

class ClusterRequest(BaseModel):
    cards: List[Card]

class ClusterInfo(BaseModel):
    category_name: str
    card_ids: List[int]

class ClusterResponse(BaseModel):
    clusters: List[ClusterInfo]

# --- 4. 모델 로딩 및 전역 변수 ---
# 서버 시작 시 모델을 메모리에 한 번만 로드하기 위해 전역 변수로 선언
models = {}

@app.on_event("startup")
def load_models():
    """서버 시작 시 AI 모델들을 로드합니다."""
    print("--- AI 모델 로딩 시작 ---")
    try:
        # Gemini 클라이언트 초기화 (환경 변수 사용)
        api_key = os.getenv("GEMINI_API_KEY")
        if not api_key:
            raise ValueError("GEMINI_API_KEY가 .env 파일에 설정되지 않았습니다.")
        models['gemini_client'] = genai.Client(api_key=api_key)
        print("Gemini 클라이언트 초기화 완료.")

        # 기타 ML 모델 로드
        models['okt'] = Okt()
        print("Okt 형태소 분석기 초기화 완료.")
        models['keybert'] = KeyBERT('distiluse-base-multilingual-cased')
        print("KeyBERT 모델 초기화 완료.")
        models['ner_pipeline'] = pipeline("ner", model="soddokayo/klue-roberta-large-klue-ner", aggregation_strategy="simple")
        print("KLUE NER 모델 초기화 완료.")
        
        print("--- 모든 AI 모델 로딩 완료 ---")
    except Exception as e:
        print(f"모델 로딩 중 치명적인 오류 발생: {e}")
        # 모델 로딩 실패 시 서버를 시작하지 않도록 처리할 수도 있음
        # 예: raise e

# --- 5. 태그 후보 추출 및 LLM 호출 함수 (기존 로직 재사용) ---

# 기능 2: 태그 생성을 위한 헬퍼 함수들
def get_tags_by_frequency(text, n_tags=20):
    nouns = models['okt'].nouns(re.sub(r'[^가-힣A-Za-z0-9\s]', '', text))
    return [n for n, c in Counter(n for n in nouns if len(n) > 1).most_common(n_tags)]

def get_tags_by_keybert_ngrams(text, n_tags=20):
    return [tag for tag, score in models['keybert'].extract_keywords(text, keyphrase_ngram_range=(1, 2), stop_words=None, top_n=n_tags)]

def get_tags_by_okt_phrases(text, n_tags=20):
    phrases = models['okt'].phrases(text)
    if not phrases: return []
    return [tag for tag, score in models['keybert'].extract_keywords(text, candidates=phrases, top_n=n_tags)]

def get_tags_by_ner(text):
    return [entity['word'].replace(" ", "") for entity in models['ner_pipeline'](text)]

# 기능 1: 클러스터링을 위한 헬퍼 함수들
def get_embeddings(texts: List[str]):
    try:
        result = models['gemini_client'].models.embed_content(
            model="gemini-embedding-001",
            contents=texts
        )
        return np.array(result.embeddings)
    except Exception as e:
        print(f"임베딩 생성 중 오류: {e}")
        return None

def name_clusters_with_llm(grouped_texts: Dict[int, List[str]]):
    cluster_names = {}
    for cluster_id, texts in grouped_texts.items():
        content_for_prompt = "\n".join([f"- {text}" for text in texts])
        prompt = f"다음은 하나의 그룹으로 묶인 문서들입니다.\n--- 문서 목록 ---\n{content_for_prompt}\n------------------\n이 문서들의 공통 주제를 가장 잘 나타내는 카테고리 이름을 2~3 단어의 명사구로 생성해주세요. 카테고리 이름만 간결하게 답변해주세요."
        try:
            response = models['gemini_client'].models.generate_content(model="gemini-pro", contents=prompt)
            cluster_names[cluster_id] = response.text.strip().replace("*", "")
        except Exception as e:
            print(f"LLM 카테고리 명명 중 오류: {e}")
            cluster_names[cluster_id] = f"카테고리 {cluster_id}"
    return cluster_names

# --- 6. API 엔드포인트 구현 ---

@app.post("/tags/generate", response_model=TagGenerationResponse)
async def generate_tags(request: TagGenerationRequest):
    """카드 내용(content)을 받아 AI 기반 태그 목록을 반환합니다."""
    content = request.content
    max_tags = 5

    # 1단계: 모든 방법론으로 태그 후보 추출
    freq_tags = get_tags_by_frequency(content)
    ngram_tags = get_tags_by_keybert_ngrams(content)
    phrase_tags = get_tags_by_okt_phrases(content)
    ner_tags = get_tags_by_ner(content)
    
    # 2단계: 모든 후보를 합치고 중복 제거
    candidate_tags = list(set(freq_tags + ngram_tags + phrase_tags + ner_tags))
    
    if not candidate_tags:
        return {"tags": []}
        
    # 3단계: LLM에게 보낼 프롬프트 (Jupyter Notebook에서 검증된 상세 버전)
    prompt = f"""
    다음은 특정 문서에서 다양한 알고리즘으로 추출한 태그 후보 목록입니다.

    --- 태그 후보 목록 ---
    {', '.join(candidate_tags)}
    --------------------
    
    이 목록을 보고, 문서의 핵심 주제를 가장 잘 나타내는 최종 태그를 {max_tags}개만 골라 다듬어주세요.
    예를 들어, '인공지능'과 'AI'가 둘 다 있다면 'AI'로 합치고, 너무 광범위하거나 중요하지 않은 단어는 제거해주세요.
    결과는 반드시쉼표(,)로만 구분된 리스트 형태로 답해주세요. (예: 태그1,태그2,태그3)
    """
    
    # 4단계: LLM 호출 (노트북에서 테스트된 모델로 변경)
    try:
        response = models['gemini_client'].models.generate_content(model="gemini-2.5-flash", contents=prompt)
        
        if "없음" in response.text:
            return {"tags": []}
            
        final_tags = [tag.strip() for tag in response.text.split(',')]
        return {"tags": final_tags}
    except Exception as e:
        print(f"태그 생성 LLM 호출 오류: {e}")
        # LLM 실패 시, 후보군 중 일부를 그냥 반환
        return {"tags": candidate_tags[:max_tags]}

@app.post("/cards/cluster", response_model=ClusterResponse)
async def cluster_cards(request: ClusterRequest):
    """전체 카드 목록을 받아 군집화하고, 각 군집에 이름을 붙여 반환합니다."""
    cards = request.cards
    card_contents = [card.content for card in cards]
    
    # 1. 벡터 변환
    card_vectors = get_embeddings(card_contents)
    if card_vectors is None:
        return {"clusters": []}
        
    # 2. 클러스터링 (K는 3으로 고정, 추후 엘보우 방식 적용 가능)
    num_clusters = min(len(cards) // 2, 5) # 간단한 K값 자동 설정 로직
    if num_clusters < 2: num_clusters = 2
    
    kmeans = KMeans(n_clusters=num_clusters, random_state=42, n_init='auto')
    cluster_labels = kmeans.fit_predict(card_vectors)
    
    # 3. 클러스터별로 카드 그룹화
    grouped_texts = {i: [] for i in range(num_clusters)}
    for i, card in enumerate(cards):
        grouped_texts[cluster_labels[i]].append(card.content)
        
    # 4. LLM으로 클러스터 이름 생성
    cluster_names = name_clusters_with_llm(grouped_texts)
    
    # 5. 최종 결과 포맷팅
    final_clusters_map = {name: [] for name in cluster_names.values()}
    for i, card in enumerate(cards):
        cluster_name = cluster_names[cluster_labels[i]]
        final_clusters_map[cluster_name].append(card.id)
        
    response_data = [{"category_name": name, "card_ids": ids} for name, ids in final_clusters_map.items()]
    
    return {"clusters": response_data}

# --- 7. 서버 실행 ---
if __name__ == "__main__":
    # 서버 실행 방법:
    # 1. 터미널에서 `pip install -r requirements.txt` 실행 (requirements.txt 파일 필요)
    # 2. `.env` 파일에 `GEMINI_API_KEY=...` 추가
    # 3. 터미널에서 `uvicorn ai_server:app --reload` 실행
    print("AI 서버를 시작합니다. http://127.0.0.1:8000 에서 실행됩니다.")
    uvicorn.run(app, host="0.0.0.0", port=8000)
