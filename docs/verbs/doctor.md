# forge doctor

Check environment health, API keys, and network mode.

## Synopsis

```
forge doctor [--json]
```

## Description

`forge doctor` validates the local environment:

- forge version
- Go version
- LLM provider detection (Anthropic / OpenAI / Gemini / Azure / Bedrock / Ollama)
- Network mode (online / air-gap)
- Plugin health
- Budget status

## Examples

```bash
forge doctor
forge doctor --json
```

## Air-gap output

```
? network: OFFLINE ? FORGE_AIRGAP=1 (mode=airgap:forced)
? LLM provider: ollama (host=http://localhost:11434)
```
