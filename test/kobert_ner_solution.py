# KoBERT-NER 최종 해결책 v2: ElectraTokenizer 직접 사용
from transformers import ElectraTokenizer, AutoModelForTokenClassification
import torch

# 1. ElectraTokenizer를 명시적으로 사용하여 토크나이저 로드
print("--- Loading ElectraTokenizer & Model ---")
tokenizer = ElectraTokenizer.from_pretrained("monologg/kocharelectra-base-modu-ner-all")
model = AutoModelForTokenClassification.from_pretrained("monologg/kocharelectra-base-modu-ner-all")
print("...Done")

def kobert_ner_final_solution(text):
    """
    KoBERT-NER 모델을 ElectraTokenizer로 실행하고, 결과를 단어 단위로 묶어주는 함수.
    """
    print(f"\n--- Running NER for: '{text}' ---")
    
    # 2. 토큰화 및 추론
    inputs = tokenizer(text, return_tensors="pt", truncation=True, max_length=512)
    tokens = tokenizer.convert_ids_to_tokens(inputs["input_ids"][0])
    
    print(f"Tokenization Check: {tokens}") # 토큰화 결과 확인
    
    with torch.no_grad():
        outputs = model(**inputs)
        predictions = torch.argmax(outputs.logits, dim=2)
        
    # 3. 결과 후처리 및 단어 단위로 묶기
    entities = []
    current_entity = None
    
    for i, token_prediction in enumerate(predictions[0]):
        label = model.config.id2label[token_prediction.item()]
        token = tokens[i]
        
        if label.startswith("B-"):
            if current_entity:
                entities.append(current_entity)
            current_entity = {"word": token.replace("##", ""), "label": label[2:]}
        elif label.startswith("I-") and current_entity:
            if current_entity["label"] == label[2:]:
                current_entity["word"] += token.replace("##", "")
            else:
                entities.append(current_entity)
                current_entity = {"word": token.replace("##", ""), "label": label[2:]}
        else:
            if current_entity:
                entities.append(current_entity)
                current_entity = None
                
    if current_entity:
        entities.append(current_entity)
        
    return entities

# --- 실행 ---
text1 = "철수는 서울역에서 비빔밥을 먹고, 삼성전자에 출근했다."
text2 = "이재용 삼성전자 회장이 다음 주 미국으로 출장을 떠난다."

results1 = kobert_ner_final_solution(text1)
print("\n--- Final Results (Text 1) ---")
for entity in results1:
    print(f"  - Entity: {entity['word']}, Label: {entity['label']}")

results2 = kobert_ner_final_solution(text2)
print("\n--- Final Results (Text 2) ---")
for entity in results2:
    print(f"  - Entity: {entity['word']}, Label: {entity['label']}")
