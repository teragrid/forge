Here is the implementation plan for the incomplete tasks for "Forge Ship v2: RFC-005 P1+P2 Token-Efficiency and Context-Budget Improvements."

---

### T-002: OpenAPI Schema Verification

#### Files to Modify/Create
- `api/v1/token_optimization.openapi.yaml` (or equivalent OpenAPI schema for the service).
- `scripts/validate_openapi.py` (utility for validation).

#### Key Steps
1. **Verify YAML Syntax in OpenAPI Definition**
   - Locate the schema in the Forge Ship API repository. Ensure proper syntax, types, and operations.
   - Check for required fields relating to `optimizeTokens` and `adjustContext`.

2. **Integrate Validations**
   - Write a script to validate the schema using an OpenAPI Validator (e.g., using the `openapi-spec-validator` library).

3. **Add CI Checks** (Optional)
   - Ensure OpenAPI schema validation runs as part of CI workflows (`.github/workflows/api-schema-check.yml`).

#### Minimal Code Example
```python
# scripts/validate_openapi.py
from openapi_spec_validator import validate_spec_url
import sys

def validate_openapi(openapi_path):
    try:
        validate_spec_url(f"file://{openapi_path}")
        print(f"✅ OpenAPI schema at {openapi_path} is valid!")
    except Exception as e:
        print(f"❌ OpenAPI Validation Error: {e}")
        sys.exit(1)

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python validate_openapi.py <path_to_openapi_yaml>")
        sys.exit(1)

    validate_openapi(sys.argv[1])
```

#### Next Steps
- Validate the OpenAPI schema locally using the script:  
  `python scripts/validate_openapi.py api/v1/token_optimization.openapi.yaml`.

---

### T-003: Setup Testing Framework

#### Files to Modify/Create
- `tests/conftest.py` (pytest configuration for fixtures and setup).
- `tests/test_metrics.py` (add benchmark tests for current implementation).

#### Key Steps
1. **Install Necessary Libraries**
   - Use `pytest`, `pytest-benchmark`, or `pytest-timing` libraries for performance measurement.
   - Install packages via pip:
     ```bash
     pip install pytest pytest-benchmark
     ```

2. **Set Up Benchmarking Metrics**
   - Include benchmarks for token usage, latency, and memory profiling.
   - Sample test cases should include various inputs at different complexity levels.

3. **Provide Report Output**
   - Generate benchmark reports in printable/parsable formats (e.g., JSON).

#### Minimal Benchmark Example
```python
# tests/test_metrics.py
import pytest
from forge_ship import process_input

@pytest.mark.benchmark
def test_token_reduction_benchmark(benchmark):
    input_data = "This is a test input with repeated words. Repeated words should be removed."
    result = benchmark(process_input, input_data)
    assert "repeated words" not in result  # Ensure redundancy is removed
```

---

### T-004: Benchmark Current Metrics

#### Files to Modify/Create
- `scripts/benchmark_baselines.py`.
- Add benchmarking configurations (`configs/benchmark_config.yaml`).

#### Key Steps
1. **Design Benchmarking Scenarios**
   - Create configurations for:
     - Low, medium, and high input token sizes.
     - Redundant vs non-redundant inputs.

2. **Extract Metrics**
   - Measure token usage before and after the input pipeline.
   - Measure average response latency with different backend loads.
   - Save results in a standardized format (CSV or JSON).

3. **Automate Benchmarking**
   - Develop a script that runs the benchmarks and aggregates results.

#### Minimal Code Example
```python
# scripts/benchmark_baselines.py
import time
from forge_ship import process_input

def benchmark():
    input_data = ["short input"] * 10 
    metrics = {"total_latency": 0, "token_reduction": 0}

    for i, text in enumerate(input_data):
        start_time = time.monotonic()
        result = process_input(text)
        end_time = time.monotonic()

        metrics["total_latency"] += (end_time - start_time)
        metrics["token_reduction"] += len(text.split()) - len(result.split())

    print(f"Average latency: {metrics['total_latency'] / len(input_data)}")
    print(f"Average token reduction: {metrics['token_reduction'] / len(input_data)}")

if __name__ == "__main__":
    benchmark()
```

---

### T-005: Implement Token Optimization API

#### Files to Modify/Create
- `api/v1/token_efficiency.py` (backend implementation of token optimization).
- Update `api/v1/token_optimization.openapi.yaml`.

#### Key Steps
1. **Define API Endpoint**
   - Route: `/api/v1/token-efficiency`.
   - HTTP Method: `POST`.

2. **Develop Optimization Function**
   - Accepts raw text input.
   - Applies token redundancy removal algorithms.

3. **Return Optimized Output**
   - Return reduced tokens along with metadata about how many tokens were reduced.

#### Minimal Code Example
```python
# api/v1/token_efficiency.py
from flask import Flask, request, jsonify
from forge_ship.optimization import optimize_tokens

app = Flask(__name__)

@app.route('/api/v1/token-efficiency', methods=['POST'])
def token_efficiency():
    input_data = request.json.get("input")
    optimized_data, metadata = optimize_tokens(input_data)

    return jsonify({
        "original_tokens": metadata["original_tokens"],
        "optimized_tokens": metadata["optimized_tokens"],
        "optimized_text": optimized_data
    })
```

#### Optimization Logic Example
```python
# forge_ship/optimization.py
def optimize_tokens(input_text):
    tokens = input_text.split()
    optimized_tokens = []
    seen_words = set()

    for token in tokens:
        if token not in seen_words:
            optimized_tokens.append(token)
            seen_words.add(token)

    original_count = len(tokens)
    optimized_count = len(optimized_tokens)
    return " ".join(optimized_tokens), {
        "original_tokens": original_count,
        "optimized_tokens": optimized_count
    }
```

---

### T-006: Optimize Context Pruning

#### Files to Modify/Create
- `forge_ship/context_pruning.py`.

#### Key Steps
1. **Define Context Rules**
   - Define heuristics to prioritize certain tokens (e.g., high utility, recency-based).

2. **Implement Recursive Pruning**
   - Given context tokens, remove least important ones while staying under the token budget.

3. **Unit Test Context Pruning**
   - Test using edge cases (e.g., long irrelevant contexts).

#### Minimal Code Example
```python
# forge_ship/context_pruning.py
def prune_context(context, max_tokens):
    tokens = context.split()
    if len(tokens) <= max_tokens:
        return context

    # Apply basic prioritization (e.g., remove least important tokens)
    while len(tokens) > max_tokens:
        tokens.pop(0)  # Remove oldest token for simplicity

    return " ".join(tokens)
```  

#### Test
```python
def test_prune_context():
    long_context = "token " * 5000  # Over 4096-token limit
    pruned = prune_context(long_context, max_tokens=4096)
    assert len(pruned.split()) == 4096
```

---

**Next Steps:**
- Progress with T-007 to T-014 as more foundational work is completed.
- Incrementally build and benchmark token-efficiency and context-budget improvements across the API and system pipelines.