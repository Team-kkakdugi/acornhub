# 최종 제안 v2: soddokayo/klue-roberta-large-klue-ner 모델 사용
# 이 셀만 복사해서 실행하시면 됩니다.

# 1. 필요한 라이브러리 설치
!pip install -q torch transformers "tokenizers>=0.13.3" "sentencepiece>=0.1.91"

from transformers import pipeline

# 2. KLUE NER 파이프라인 로드 (정확한 모델 이름 사용)
try:
    print("--- Loading KLUE NER Model (soddokayo/klue-roberta-large-klue-ner) ---")
    ner_pipeline = pipeline("ner", model="soddokayo/klue-roberta-large-klue-ner", aggregation_strategy="simple")
    print("...Model Loaded Successfully!")
except Exception as e:
    print(f"An error occurred during model loading: {e}")
    ner_pipeline = None

# 3. 실행 및 결과 확인
if ner_pipeline:
    text = "철수는 서울역에서 비빔밥을 먹고, 삼성전자에 출근했다."
    print(f"\nInput: {text}")
    
    ner_results = ner_pipeline(text)
    
    print("\n--- NER Results ---")
    if ner_results:
        for entity in ner_results:
            print(f"  - Entity: {entity['word']}, Label: {entity['entity_group']}, Score: {entity['score']:.4f}")
    else:
        print("No entities found.")