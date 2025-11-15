import os
import re
import json
import warnings
from collections import Counter

import uvicorn
import anthropic
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from typing import List, Dict, Any

import numpy as np
from sklearn.cluster import KMeans
import google.genai as genai
from dotenv import load_dotenv

from konlpy.tag import Okt
from keybert import KeyBERT
from transformers import pipeline

warnings.filterwarnings("ignore", category=FutureWarning)
load_dotenv()

app = FastAPI()

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

class TagGenerationRequest(BaseModel):
    content: str

class TagGenerationResponse(BaseModel):
    tags: List[str]

class CategoryInfo(BaseModel):
    category_name: str
    card_ids: List[int]

class AgentInvokeRequest(BaseModel):
    topic: str
    all_tags: List[str]
    all_categories: List[CategoryInfo]
    all_cards: List[Card]

class AgentInvokeResponse(BaseModel):
    report: str

models = {}
claude_client = None

@app.on_event("startup")
def load_models():
    """서버 시작 시 AI 모델들을 로드합니다."""
    global claude_client
    print("--- AI 모델 로딩 시작 ---")
    try:
        gemini_api_key = os.getenv("GEMINI_API_KEY")
        if gemini_api_key:
            models['gemini_client'] = genai.Client(api_key=gemini_api_key)
            print("Gemini 클라이언트 초기화 완료.")
        else:
            print("경고: GEMINI_API_KEY가 없어 Gemini 관련 기능이 제한됩니다.")
            models['gemini_client'] = None

        anthropic_api_key = os.getenv("ANTHROPIC_API_KEY")
        if anthropic_api_key:
            claude_client = anthropic.Anthropic(api_key=anthropic_api_key)
            print("Anthropic (Claude) 클라이언트 초기화 완료.")
        else:
            print("경고: ANTHROPIC_API_KEY가 없어 Claude 관련 기능이 제한됩니다.")

        models['okt'] = Okt()
        print("Okt 형태소 분석기 초기화 완료.")
        models['keybert'] = KeyBERT('distiluse-base-multilingual-cased')
        print("KeyBERT 모델 초기화 완료.")
        models['ner_pipeline'] = pipeline("ner", model="soddokayo/klue-roberta-large-klue-ner", aggregation_strategy="simple")
        print("KLUE NER 모델 초기화 완료.")
        
        print("--- 모든 AI 모델 로딩 완료 ---")
    except Exception as e:
        print(f"모델 로딩 중 치명적인 오류 발생: {e}")
        raise e

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

def get_embeddings(texts: List[str]):
    gemini_client = models.get('gemini_client')
    if not gemini_client:
        print("임베딩 오류: Gemini 클라이언트가 없습니다.")
        return None
    try:
        result = gemini_client.models.embed_content(
            model="gemini-embedding-001",
            contents=texts
        )

        embedding_list = [embedding.values for embedding in result.embeddings]
        return np.array(embedding_list)
    
    except Exception as e:
        print(f"임베딩 생성 중 오류: {e}")
        return None

def name_clusters_with_llm(grouped_texts: Dict[int, List[str]]):
    cluster_names = {}
    gemini_client = models.get('gemini_client')
    if not gemini_client: return {i: f"카테고리 {i+1}" for i in grouped_texts.keys()}
    
    for cluster_id, texts in grouped_texts.items():
        print(f"\n--- 클러스터 {cluster_id}의 카테고리 이름 생성 중... ---")
        content_for_prompt = "\n".join([f"- {text}" for text in texts])
        prompt = f"""
        다음은 하나의 그룹으로 묶인 문서들입니다.
        --- 문서 목록 ---
        {content_for_prompt}
        ------------------
        이 문서들의 공통 주제를 가장 잘 나타내는 카테고리 이름을 2~3 단어의 명사구로 생성해주세요.
        (예: AI 기술 동향, 클라우드 아키텍처, 반도체 시장)
        카테고리 이름만 간결하게 답변해주세요.
        """
        try:
            response = gemini_client.models.generate_content(model="gemini-2.5-flash", contents=prompt)
            cluster_name = response.text.strip().replace("*", "")
            cluster_names[cluster_id] = cluster_name
            print(f"생성된 이름: {cluster_name}")
        except Exception as e:
            print(f"LLM 호출 중 오류: {e}")
            cluster_names[cluster_id] = f"카테고리 {cluster_id}"
    
    return cluster_names

@app.post("/tags/generate", response_model=TagGenerationResponse)
async def generate_tags(request: TagGenerationRequest):
    content = request.content
    max_tags = 5
    gemini_client = models.get('gemini_client')
    if not gemini_client: raise HTTPException(status_code=503, detail="Gemini 모델이 로드되지 않았습니다.")

    candidate_tags = list(set(
        get_tags_by_frequency(content) + get_tags_by_keybert_ngrams(content) +
        get_tags_by_okt_phrases(content) + get_tags_by_ner(content)
    ))
    if not candidate_tags: return {"tags": []}
        
    prompt = f"""
    다음은 특정 문서에서 다양한 알고리즘으로 추출한 태그 후보 목록입니다.

    --- 태그 후보 목록 ---
    {', '.join(candidate_tags)}
    --------------------
    
    이 목록을 보고, 문서의 핵심 주제를 가장 잘 나타내는 최종 태그를 {max_tags}개만 골라 다듬어주세요.
    예를 들어, '인공지능'과 'AI'가 둘 다 있다면 'AI'로 합치고, 너무 광범위하거나 중요하지 않은 단어는 제거해주세요.
    결과는 반드시쉼표(,)로만 구분된 리스트 형태로 답해주세요. (예: 태그1,태그2,태그3)
    """

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
    cards = request.cards
    if len(cards) < 2: return {"clusters": [{"category_name": "미분류", "card_ids": [c.id for c in cards]}]}

    card_contents = [card.content for card in cards]
    card_vectors = get_embeddings(card_contents)
    if card_vectors is None: raise HTTPException(status_code=500, detail="카드 벡터 임베딩 생성 실패.")
        
    num_clusters = min(len(cards) // 2, 5)
    if num_clusters < 2: num_clusters = 2
    
    kmeans = KMeans(n_clusters=num_clusters, random_state=42, n_init='auto')
    cluster_labels = kmeans.fit_predict(card_vectors)
    
    grouped_texts = {i: [] for i in range(num_clusters)}
    for i, card in enumerate(cards):
        grouped_texts[cluster_labels[i]].append(card.content)
        
    cluster_names = name_clusters_with_llm(grouped_texts)
    
    final_clusters_map = {name: [] for name in cluster_names.values()}
    for i, card in enumerate(cards):
        cluster_name = cluster_names[cluster_labels[i]]
        final_clusters_map[cluster_name].append(card.id)
        
    response_data = [{"category_name": name, "card_ids": ids} for name, ids in final_clusters_map.items()]
    return {"clusters": response_data}

card_db_for_agent = {}

def search_cards_for_agent(categories: list[str], all_categories: List[Dict]) -> list[dict]:
    print(f"\n[Tool Call] search_cards(categories={categories})")
    found_card_ids = set()
    if categories and all_categories:
        for cat_info in all_categories:
            if cat_info['category_name'] in categories:
                found_card_ids.update(cat_info['card_ids'])
    
    results = [{"id": cid, "content": card_db_for_agent.get(cid, "내용 없음")} for cid in found_card_ids]
    print(f"[Tool Result] Found {len(results)} cards.")
    return results

@app.post("/agent/invoke", response_model=AgentInvokeResponse)
async def invoke_agent(request: AgentInvokeRequest):
    if not claude_client: raise HTTPException(status_code=503, detail="Claude 모델이 로드되지 않았습니다.")

    global card_db_for_agent
    card_db_for_agent = {card.id: card.content for card in request.all_cards}

    tools = [{"name": "search_cards", "description": "관련 카드 내용을 검색합니다.", "input_schema": {
        "type": "object", "properties": {
            "categories": {"type": "array", "items": {"type": "string"}, "description": "검색할 카테고리 이름 목록"}
        }, "required": ["categories"]
    }}]

    prompt = f"""당신은 전문 보고서 작성 AI 에이전트입니다.
    ## 최종 목표: '{request.topic}'에 대한 보고서 초안을 '개요', '서론', '본론', '결론' 구조로 **HTML 형식**으로 작성하세요.
    - 각 섹션 제목('개요', '서론', '본론', '결론')은 `<h2>` 태그로 감싸세요.
    - 모든 문단은 `<p>` 태그로 감싸세요.
    - 최종 결과물은 앞 뒤 다른 설명 없이 완전한 HTML 코드여야 합니다.
    - 다만, html 에서 head, body 등의 태그는 제외하며, 오직 내용 부분만 작성하세요.
    ## 사용 가능 정보:
    - 전체 태그: {request.all_tags}
    - 전체 카테고리: {[cat.category_name for cat in request.all_categories]}
    ## 작업 절차:
    1. 주제와 가장 관련 높은 카테고리를 선택해 `search_cards` 함수로 정보를 수집.
    2. 수집된 정보들을 종합하여 최종 보고서를 논리적인 HTML 형식으로 작성. 보고서 외 불필요한 설명은 제외.
    """
    messages = [{"role": "user", "content": prompt}]
    
    try:
        for _ in range(3):
            response = claude_client.messages.create(
                model="claude-sonnet-4-5-20250929", max_tokens=4096, tools=tools, messages=messages
            )
            messages.append({"role": "assistant", "content": response.content})
            if response.stop_reason != "tool_use": break

            tool_results = []
            for tool_block in [b for b in response.content if b.type == "tool_use"]:
                if tool_block.name == "search_cards":
                    result = search_cards_for_agent(
                        categories=tool_block.input.get("categories"),
                        all_categories=[c.dict() for c in request.all_categories]
                    )
                    tool_results.append({"type": "tool_result", "tool_use_id": tool_block.id, "content": json.dumps(result, ensure_ascii=False)})
            messages.append({"role": "user", "content": tool_results})

        final_text = next((b.text for b in response.content if b.type == 'text'), "최종 보고서를 생성하지 못했습니다.")
        return {"report": final_text}
    except Exception as e:
        print(f"Claude 에이전트 실행 중 오류: {e}")
        raise HTTPException(status_code=500, detail=f"AI 에이전트 실행 중 오류 발생: {e}")

if __name__ == "__main__":
    print("AI 서버를 시작합니다. http://127.0.0.1:8000 에서 실행됩니다.")
    uvicorn.run(app, host="0.0.0.0", port=8000)