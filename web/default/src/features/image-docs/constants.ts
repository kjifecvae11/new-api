/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

export const GPT_IMAGE_CURL = `curl https://ainiubi.org/v1/images/generations \\
  -H "Authorization: Bearer $NEWAPI_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-image-2",
    "prompt": "A modern transit control room, cinematic blue light, no text",
    "size": "1024x1024",
    "quality": "low",
    "output_format": "png",
    "n": 1
  }'`

export const GROK_IMAGE_CURL = `curl https://ainiubi.org/v1/images/generations \\
  -H "Authorization: Bearer $NEWAPI_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "grok-imagine-image-quality",
    "prompt": "A modern transit control room, cinematic blue light, no text",
    "aspect_ratio": "16:9",
    "resolution": "1k",
    "response_format": "b64_json",
    "n": 1
  }'`

export const RESPONSE_EXAMPLE = `{
  "created": 1785900000,
  "data": [
    { "b64_json": "iVBORw0KGgoAAA..." }
  ]
}`

export const JAVASCRIPT_EXAMPLE = `import { writeFile } from "node:fs/promises";

const model = process.env.IMAGE_MODEL || "gpt-image-2";
const grok = model.startsWith("grok-");
const body = {
  model,
  prompt: "A modern transit control room, cinematic blue light, no text",
  n: 1,
  ...(grok
    ? { aspect_ratio: "16:9", resolution: "1k", response_format: "b64_json" }
    : { size: "1536x1024", quality: "low", output_format: "png" }),
};

const response = await fetch("https://ainiubi.org/v1/images/generations", {
  method: "POST",
  headers: {
    Authorization: \`Bearer \${process.env.NEWAPI_KEY}\`,
    "Content-Type": "application/json",
  },
  body: JSON.stringify(body),
  signal: AbortSignal.timeout(180_000),
});
const result = await response.json();
if (!response.ok) throw new Error(result?.error?.message || \`HTTP \${response.status}\`);
await writeFile("generated-image.png", Buffer.from(result.data[0].b64_json, "base64"));`

export const CODEX_CONFIG = `model = "gpt-5.6-sol"
model_provider = "newapi-production"

[model_providers.newapi-production]
name = "NewAPI production relay"
base_url = "https://ainiubi.org/v1"
wire_api = "responses"
requires_openai_auth = true
env_http_headers = { "X-NewAPI-Key" = "NEWAPI_KEY" }`

export const CODEX_DIRECT_PROMPT = `Read the NewAPI key from the NEWAPI_KEY environment variable.
Call POST https://ainiubi.org/v1/images/generations with the requested image model.
Never print or log the key. Decode data[0].b64_json, save the image, and return its path.`

export const MODEL_LIST_CURL = `curl https://ainiubi.org/v1/models \\
  -H "Authorization: Bearer $NEWAPI_KEY"`
