# 图片生成 API 使用指南

本文面向 `https://ainiubi.org` 的 NewAPI Key 用户。无需登录即可访问的网页版文档位于 [https://ainiubi.org/docs/images](https://ainiubi.org/docs/images)。用户只需自己的 NewAPI Key；Codex OAuth 文件、上游 access token 和 account ID 均由服务端保管，不应由用户上传或传入接口。

## 快速信息

- Base URL：`https://ainiubi.org/v1`
- 推荐图片模型：`gpt-image-2` 或 `grok-imagine-image-quality`
- 可用模型：`GET /v1/models`
- 普通 SDK / curl：`Authorization: Bearer <NEWAPI_KEY>`
- Codex 分离认证：`X-NewAPI-Key: <NEWAPI_KEY>`

图片模型统一使用 `POST /v1/images/generations`。客户端应保持认证、超时、Base64 解码和错误处理逻辑不变，只按所选模型切换参数块；后续接入的新图片模型以认证后的 `GET /v1/models` 和网页版能力说明为准。

| 场景 | 接口 | 图片 Base64 位置 |
| --- | --- | --- |
| 根据提示词直接生成图片 | `POST /v1/images/generations` | `data[].b64_json` |
| 智能体理解上下文后生成图片 | `POST /v1/responses`（服务端自动提供 `image_generation`） | `output[].result` |

## Images API

```bash
curl https://ainiubi.org/v1/images/generations \
  -H "Authorization: Bearer $NEWAPI_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一辆未来感有轨电车驶入雨夜中的霓虹车站，电影级灯光",
    "size": "1024x1024",
    "quality": "low",
    "output_format": "png",
    "n": 1
  }'
```

成功响应中的 `b64_json` 是图片内容，不是 URL：

```json
{
  "created": 1785900000,
  "data": [{ "b64_json": "iVBORw0KGgoAAA..." }]
}
```

Python SDK 示例：

```python
import base64
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["NEWAPI_KEY"],
    base_url="https://ainiubi.org/v1",
)
response = client.images.generate(
    model="gpt-image-2",
    prompt="一辆未来感有轨电车驶入雨夜中的霓虹车站，电影级灯光",
    size="1024x1024",
    quality="low",
    output_format="png",
    n=1,
)
with open("tram.png", "wb") as image_file:
    image_file.write(base64.b64decode(response.data[0].b64_json))
```

### Grok Imagine

Grok 图片同样使用 NewAPI Key，不需要用户提供 Grok 登录或 xAI API Key。服务端通过受保护的 Grok CLI 会话生成图片，将临时 JPEG 转成 Base64 后立即删除服务端文件：

```bash
curl https://ainiubi.org/v1/images/generations \
  -H "Authorization: Bearer $NEWAPI_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "grok-imagine-image-quality",
    "prompt": "一辆未来感有轨电车驶入雨夜中的霓虹车站，电影级灯光",
    "aspect_ratio": "16:9",
    "resolution": "1k",
    "response_format": "b64_json",
    "n": 1
  }'
```

当前 Grok CLI 后端仅支持 `n: 1`、`resolution: "1k"` 和 `response_format: "b64_json"`。支持的画幅包括 `1:1`、`16:9`、`9:16`、`4:3`、`3:4`、`3:2`、`2:3`、`2:1`、`1:2` 和 `auto`。Grok 图片接口不使用 GPT 图片模型的 `size`、`quality` 或 `output_format` 参数。

常用参数：

| 参数 | 必填 | 建议 |
| --- | --- | --- |
| `model` | 是 | 使用 `gpt-image-2` |
| `prompt` | 是 | 写明主体、环境、风格、构图和是否需要文字 |
| `n` | 否 | 初次调用使用 `1` |
| `size` | 否 | 默认可用 `1024x1024`，横竖图尺寸按模型能力选择 |
| `quality` | 否 | 草稿 `low`，成品 `medium` / `high` |
| `output_format` | 否 | `png`、`jpeg` 或 `webp` |
| `output_compression` | 否 | `jpeg` / `webp` 可使用 `0`–`100` |

## Responses API 图片工具

Responses API 适合 Codex 或其他智能体。对于走 Codex OAuth 渠道的普通响应请求，服务端会在不修改用户 Key 和客户端配置的情况下自动补充 `image_generation` 工具；模型根据自然语言自行决定是否生成图片。客户端原有工具和 `tool_choice` 会保留，已经声明图片工具时也不会重复添加。

因此，已经把模型请求指向 `https://ainiubi.org/v1` 的现有用户只需像原来一样发送自然语言，不需要增加 `tools` 字段：

```bash
curl https://ainiubi.org/v1/responses \
  -H "Authorization: Bearer $NEWAPI_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4-mini",
    "input": "生成一张现代化中转站控制中心的横版图片，不要文字"
  }'
```

文字模型请先通过 `/v1/models` 确认可用，示例模型不是固定默认值。需要指定尺寸、质量等高级选项时，客户端仍可显式传入 `tools: [{"type":"image_generation", ...}]`，服务端会原样保留。

从输出中找到 `type` 为 `image_generation_call` 的项目，将其 `result` 字段做 Base64 解码：

```python
import base64
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["NEWAPI_KEY"],
    base_url="https://ainiubi.org/v1",
)
response = client.responses.create(
    model="gpt-5.4-mini",
    input="生成一张现代化中转站控制中心的横版图片，不要文字",
)
image_call = next(
    item for item in response.output
    if item.type == "image_generation_call"
)
with open("control-center.png", "wb") as image_file:
    image_file.write(base64.b64decode(image_call.result))
```

## Codex 分离认证

Codex 的内置工具可能需要继续使用现有 ChatGPT 登录，因此不要用 NewAPI Key 覆盖 Codex 的 `Authorization`。将 NewAPI Key 放入环境变量，并让发往自定义 provider 的请求额外携带 `X-NewAPI-Key`：

```toml
model = "gpt-5.6-sol"
model_provider = "newapi-production"

[model_providers.newapi-production]
name = "NewAPI production relay with Codex built-in tools"
base_url = "https://ainiubi.org/v1"
wire_api = "responses"
requires_openai_auth = true
env_http_headers = { "X-NewAPI-Key" = "NEWAPI_KEY" }
```

已经使用 `https://ainiubi.org/v1` 和原有 NewAPI Key 的用户不需要修改配置。直接对 Codex 说“生成一张……图片”即可；到达 `/v1/responses` 的请求会由服务端获得图片工具，生成结果通过标准 Responses 事件返回。普通 Bearer Key 客户端继续使用原来的 `Authorization`，无需改成 `X-NewAPI-Key`。

Codex 产品自身的内置图片功能可能使用独立的 ChatGPT 认证链路；本服务保证的是所有到达 `ainiubi.org/v1/responses` 的 Codex 渠道请求自动具备图片生成能力。

## 使用规范

1. Key 只放在服务端、环境变量或密钥存储中，不得写入前端、仓库、日志或聊天内容。
2. 不得向用户分发服务端 OAuth JSON、access token 或 account ID。
3. 图片响应可能较大，客户端超时建议不少于 180 秒；Base64 解码后再保存或上传。
4. 不要长期记录完整的 `b64_json` / `result`。
5. 对 `429` 和 `5xx` 使用带抖动的指数退避；对参数、认证、内容策略或用户错误先修正请求，不要原样循环重试。
6. 初次调用建议 `n: 1`、`quality: low`，确认提示词后再提高数量或质量。
7. 必须遵守平台内容政策及适用法律，不得生成违法、侵权、欺诈或其他禁止内容。

## 常见错误

| HTTP 状态 | 处理方式 |
| --- | --- |
| `400` | 检查 `prompt`、模型、尺寸和格式；不要把 Base64 当 URL |
| `401` | 检查 Bearer 或 `X-NewAPI-Key`；不要传服务端 OAuth |
| `403` | 联系管理员检查用户状态和模型授权 |
| `429` | 检查额度或速率限制，随后退避重试 |
| `5xx` | 保留请求标识并退避重试；持续失败时联系管理员 |

当前服务端 OAuth 通道保证图片生成接口可用。`/v1/images/edits` 等编辑接口不能假定使用同一通道，除非后续公告明确说明支持。

更多通用接口请参考 [NewAPI 官方 API 文档](https://docs.newapi.pro/zh/docs/api)。图片参数的上游定义可参考 [OpenAI Image generation guide](https://developers.openai.com/api/docs/guides/image-generation)。实际可用模型与配额以本站 `/v1/models` 和管理员配置为准。
