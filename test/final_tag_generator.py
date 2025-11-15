# 최종 v5: 모든 추출 방식을 종합하여 후보군 생성
# 이 셀의 코드를 복사해서 노트북 마지막에 추가하고 실행하세요.

# --- 1. 기본 라이브러리 설치 및 임포트 ---
!pip install -q -U google-genai konlpy keybert "transformers>=4.31.0" "tokenizers>=0.13.3" "sentencepiece>=0.1.91"

import re
from collections import Counter
import google.genai as genai
from konlpy.tag import Okt
from keybert import KeyBERT
from transformers import pipeline
import torch

print("--- 라이브러리 임포트 완료 ---")

# --- 2. 모델 초기화 ---

# Gemini LLM 클라이언트
try:
    client = genai.Client(api_key="AIzaSyBhLmkl8pch1-aUmi3VsCsDRDYYjCgI2Dk")
    print("Gemini LLM 클라이언트 초기화 완료.")
except Exception as e:
    print(f"Gemini LLM 클라이언트 초기화 실패: {e}")
    client = None

# Okt
try:
    okt = Okt()
    print("Okt 형태소 분석기 초기화 완료.")
except Exception as e:
    print(f"Okt 초기화 실패: {e}")
    okt = None

# KeyBERT
try:
    kw_model = KeyBERT('distiluse-base-multilingual-cased')
    print("KeyBERT 모델 초기화 완료.")
except Exception as e:
    print(f"KeyBERT 초기화 실패: {e}")
    kw_model = None

# KLUE NER
try:
    ner_pipeline = pipeline("ner", model="soddokayo/klue-roberta-large-klue-ner", aggregation_strategy="simple")
    print("KLUE NER 모델 초기화 완료.")
except Exception as e:
    print(f"KLUE NER 초기화 실패: {e}")
    ner_pipeline = None
    
print("--- 모든 모델 초기화 완료 ---")


# --- 3. 태그 후보 추출 함수들 (모든 방법 총동원) ---

def get_tags_by_frequency(text, n_tags=20):
    if not okt: return []
    nouns = okt.nouns(re.sub(r'[^가-힣A-Za-z0-9\s]', '', text))
    return [n for n, c in Counter(n for n in nouns if len(n) > 1).most_common(n_tags)]

def get_tags_by_keybert_ngrams(text, n_tags=20):
    if not kw_model: return []
    return [tag for tag, score in kw_model.extract_keywords(text, keyphrase_ngram_range=(1, 2), stop_words=None, top_n=n_tags)]

def get_tags_by_okt_phrases(text, n_tags=20):
    if not kw_model or not okt: return []
    phrases = okt.phrases(text)
    if not phrases: return []
    return [tag for tag, score in kw_model.extract_keywords(text, candidates=phrases, top_n=n_tags)]

def get_tags_by_ner(text):
    if not ner_pipeline: return []
    return [entity['word'].replace(" ", "") for entity in ner_pipeline(text)]


# --- 4. LLM을 이용한 최종 태그 선택 함수 ---

def generate_final_tags_with_llm(card_content, max_tags=5):
    if not client:
        print("LLM 클라이언트가 초기화되지 않아 태그 생성을 건너뜁니다.")
        return []

    # 1단계: 모든 방법론으로 태그 후보 추출
    print("\n--- 1. 태그 후보 추출 중... ---")
    freq_tags = get_tags_by_frequency(card_content)
    ngram_tags = get_tags_by_keybert_ngrams(card_content)
    phrase_tags = get_tags_by_okt_phrases(card_content)
    ner_tags = get_tags_by_ner(card_content)
    
    # 2단계: 모든 후보를 합치고 중복 제거
    candidate_tags = list(set(freq_tags + ngram_tags + phrase_tags + ner_tags))
    
    print(f"\n--- 2. 태그 후보 종합 (총 {len(candidate_tags)}개) ---")
    print(candidate_tags)
    
    if not candidate_tags:
        print("추출된 태그 후보가 없습니다.")
        return []
        
    # 3단계: LLM에게 보낼 프롬프트
    prompt = f"""
    다음은 특정 문서에서 다양한 알고리즘으로 추출한 태그 후보 목록입니다.

    --- 태그 후보 목록 ---
    {', '.join(candidate_tags)}
    --------------------
    
    이 목록을 보고, 문서의 핵심 주제를 가장 잘 나타내는 최종 태그를 {max_tags}개만 골라 다듬어주세요.
    예를 들어, '인공지능'과 'AI'가 둘 다 있다면 'AI'로 합치고, 너무 광범위하거나 중요하지 않은 단어는 제거해주세요.
    결과는 반드시쉼표(,)로만 구분된 리스트 형태로 답해주세요. (예: 태그1,태그2,태그3)
    """
    
    # 4단계: LLM 호출
    print(f"\n--- 3. Gemini-Pro에게 최적 태그 선택/정리 요청 ({max_tags}개) ---")
    try:
        response = client.models.generate_content(model="gemini-pro", contents=prompt)
        final_tags = [tag.strip() for tag in response.text.split(',')]
        return final_tags
    except Exception as e:
        print(f"LLM 호출 중 오류 발생: {e}")
        return []

# --- 5. 실행 ---
card_text = """
인공지능(AI) 기술이 빠르게 발전하면서, 자연어 처리(NLP) 분야도 큰 변화를 맞이하고 있습니다.
최근 구글, 오픈AI 등 빅테크 기업들은 초거대 AI 모델을 연이어 발표했습니다.
이러한 모델들은 작문, 번역, 코딩 등 다양한 영역에서 인간 수준의 능력을 보여주며
산업 전반에 걸쳐 혁신을 주도하고 있습니다. 하지만 모델의 편향성 문제와
환경 비용에 대한 우려도 함께 제기되고 있어, 책임감 있는 AI 개발이 중요한 화두로 떠올랐습니다.
이재용 삼성전자 회장은 AI 인재 확보의 중요성을 강조했습니다.
"""

final_tags = generate_final_tags_with_llm(card_text, max_tags=5)

print("\n" + "="*20)
print(" 최종 생성 태그")
print("="*20)
print(final_tags)
