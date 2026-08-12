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
import {
  ImageIcon,
  KeyRoundIcon,
  ShieldCheckIcon,
  WorkflowIcon,
} from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'
import { CodeBlock } from './code-block'
import {
  CODEX_CONFIG,
  CODEX_DIRECT_PROMPT,
  GPT_IMAGE_CURL,
  GROK_IMAGE_CURL,
  JAVASCRIPT_EXAMPLE,
  MODEL_LIST_CURL,
  RESPONSE_EXAMPLE,
} from './constants'

function SectionTitle(props: {
  id: string
  title: string
  description: string
}) {
  return (
    <div className='mb-6 scroll-mt-24' id={props.id}>
      <h2 className='text-2xl font-semibold tracking-tight md:text-3xl'>
        {props.title}
      </h2>
      <p className='text-muted-foreground mt-2 max-w-3xl leading-7'>
        {props.description}
      </p>
    </div>
  )
}

export function ImageDocs() {
  const { t } = useTranslation()
  const quickFacts: Array<{ icon: LucideIcon; label: string; value: string }> =
    [
      {
        icon: KeyRoundIcon,
        label: t('Authentication'),
        value: 'Authorization: Bearer NEWAPI_KEY',
      },
      {
        icon: ImageIcon,
        label: t('Endpoint'),
        value: 'POST /v1/images/generations',
      },
      { icon: ShieldCheckIcon, label: t('Result'), value: 'data[0].b64_json' },
    ]

  return (
    <PublicLayout showMainContainer={false}>
      <main>
        <section className='border-border/60 bg-muted/20 border-b pt-28 pb-14'>
          <div className='mx-auto max-w-6xl px-6'>
            <div className='flex flex-wrap gap-2'>
              <Badge variant='secondary'>{t('Public documentation')}</Badge>
              <Badge variant='outline'>{t('No sign-in required')}</Badge>
            </div>
            <h1 className='mt-6 max-w-4xl text-4xl font-semibold tracking-tight text-balance md:text-6xl'>
              {t('Image generation API for NewAPI developers')}
            </h1>
            <p className='text-muted-foreground mt-5 max-w-3xl text-lg leading-8'>
              {t(
                'Use one OpenAI-compatible endpoint and one NewAPI key to access GPT, Grok, and future image models. Provider-specific options remain explicit and predictable.'
              )}
            </p>
            <div className='mt-8 flex flex-wrap items-center gap-3 font-mono text-sm'>
              <span className='bg-primary text-primary-foreground rounded-md px-3 py-2 font-semibold'>
                POST
              </span>
              <code className='border-border bg-background rounded-md border px-3 py-2'>
                https://ainiubi.org/v1/images/generations
              </code>
            </div>
          </div>
        </section>

        <div className='mx-auto grid max-w-6xl gap-12 px-6 py-14 lg:grid-cols-[220px_minmax(0,1fr)]'>
          <aside className='hidden lg:block'>
            <nav
              className='sticky top-24 space-y-1 text-sm'
              aria-label={t('On this page')}
            >
              {[
                ['quick-start', t('Quick start')],
                ['models', t('Models and parameters')],
                ['response', t('Response format')],
                ['adapter', t('Provider-neutral adapter')],
                ['codex', t('Codex integration')],
                ['errors', t('Errors and limits')],
              ].map((item) => (
                <a
                  key={item[0]}
                  href={`#${item[0]}`}
                  className='text-muted-foreground hover:bg-muted hover:text-foreground block rounded-md px-3 py-2 transition-colors'
                >
                  {item[1]}
                </a>
              ))}
            </nav>
          </aside>

          <div className='min-w-0 space-y-16'>
            <section>
              <SectionTitle
                id='quick-start'
                title={t('Quick start')}
                description={t(
                  'Send the user-owned NewAPI key as a Bearer token. Upstream credentials stay on the server and are never exposed to clients.'
                )}
              />
              <div className='grid gap-4 md:grid-cols-3'>
                {quickFacts.map((fact) => {
                  const Icon = fact.icon
                  return (
                    <Card key={fact.label}>
                      <CardHeader className='pb-2'>
                        <CardTitle className='flex items-center gap-2 text-sm'>
                          <Icon aria-hidden='true' className='size-4' />
                          {fact.label}
                        </CardTitle>
                      </CardHeader>
                      <CardContent>
                        <code className='text-muted-foreground text-xs break-all'>
                          {fact.value}
                        </code>
                      </CardContent>
                    </Card>
                  )
                })}
              </div>
            </section>

            <section>
              <SectionTitle
                id='models'
                title={t('Models and parameters')}
                description={t(
                  'The endpoint is shared, but each provider accepts its own image options. Do not send GPT size and quality fields to Grok, or Grok aspect ratio and resolution fields to GPT.'
                )}
              />
              <div className='grid gap-8'>
                <div>
                  <div className='mb-3 flex flex-wrap items-center gap-2'>
                    <h3 className='text-xl font-semibold'>GPT Image</h3>
                    <Badge>gpt-image-2</Badge>
                  </div>
                  <p className='text-muted-foreground mb-4 text-sm'>
                    {t(
                      'Supports size, quality, output format, compression, and image count according to the active server channel.'
                    )}
                  </p>
                  <CodeBlock language='curl' code={GPT_IMAGE_CURL} />
                </div>
                <div>
                  <div className='mb-3 flex flex-wrap items-center gap-2'>
                    <h3 className='text-xl font-semibold'>Grok Imagine</h3>
                    <Badge>grok-imagine-image-quality</Badge>
                  </div>
                  <p className='text-muted-foreground mb-4 text-sm'>
                    {t(
                      'The current server-backed Grok route requires n=1, resolution=1k, and response_format=b64_json. Common aspect ratios include 1:1, 16:9, 9:16, 4:3, 3:4, and auto.'
                    )}
                  </p>
                  <CodeBlock language='curl' code={GROK_IMAGE_CURL} />
                </div>
              </div>
              <p className='text-muted-foreground mt-5 text-sm'>
                {t(
                  'Query the authenticated model list before choosing a model; newly deployed image providers will appear there.'
                )}
              </p>
              <div className='mt-3'>
                <CodeBlock language='curl' code={MODEL_LIST_CURL} />
              </div>
            </section>

            <section>
              <SectionTitle
                id='response'
                title={t('Response format')}
                description={t(
                  'Successful image responses use the OpenAI-compatible data array. Decode b64_json before saving or uploading the image; it is not a URL.'
                )}
              />
              <CodeBlock language='json' code={RESPONSE_EXAMPLE} />
            </section>

            <section>
              <SectionTitle
                id='adapter'
                title={t('Provider-neutral adapter')}
                description={t(
                  'Keep one client endpoint and select only the provider-specific option block. This makes future image models easier to add without changing authentication or response handling.'
                )}
              />
              <CodeBlock language='JavaScript' code={JAVASCRIPT_EXAMPLE} />
            </section>

            <section>
              <SectionTitle
                id='codex'
                title={t('Codex integration')}
                description={t(
                  'Codex may need its existing ChatGPT authorization and a separate NewAPI key at the same time. Preserve Codex authorization and add the NewAPI key through X-NewAPI-Key.'
                )}
              />
              <CodeBlock language='TOML' code={CODEX_CONFIG} />
              <div className='border-border bg-muted/30 mt-6 rounded-xl border p-5'>
                <div className='flex items-center gap-2 font-medium'>
                  <WorkflowIcon aria-hidden='true' className='size-4' />
                  {t('Choosing the image path in Codex')}
                </div>
                <ul className='text-muted-foreground mt-3 list-disc space-y-2 pl-5 text-sm leading-6'>
                  <li>
                    {t(
                      'For GPT image generation through a Codex-compatible channel, the built-in image flow uses gpt-image-2.'
                    )}
                  </li>
                  <li>
                    {t(
                      'To select Grok or another image provider explicitly, instruct the agent to call the Images API with an HTTP or shell tool and decode the Base64 response.'
                    )}
                  </li>
                  <li>
                    {t(
                      'Never place the NewAPI key in prompts, source files, browser code, or logs. Use an environment variable or secret store.'
                    )}
                  </li>
                </ul>
              </div>
              <div className='mt-5'>
                <CodeBlock
                  language={t('Agent prompt')}
                  code={CODEX_DIRECT_PROMPT}
                />
              </div>
            </section>

            <section>
              <SectionTitle
                id='errors'
                title={t('Errors and limits')}
                description={t(
                  'Use a client timeout of at least 180 seconds. Retry temporary failures with jittered exponential backoff and honor Retry-After when present.'
                )}
              />
              <div className='border-border overflow-hidden rounded-xl border'>
                {[
                  [
                    '400',
                    t(
                      'Invalid model or provider-specific parameters. Fix the request before retrying.'
                    ),
                  ],
                  ['401', t('Missing, invalid, or expired NewAPI key.')],
                  [
                    '403',
                    t('The user, group, channel, or model is not authorized.'),
                  ],
                  [
                    '429',
                    t(
                      'Quota, rate, concurrency, or upstream demand limit. Back off before retrying.'
                    ),
                  ],
                  [
                    '5xx',
                    t(
                      'Temporary gateway or upstream failure. Keep the request identifier and retry safely.'
                    ),
                  ],
                ].map((row) => (
                  <div
                    key={row[0]}
                    className='border-border grid grid-cols-[70px_1fr] border-b p-4 text-sm last:border-b-0'
                  >
                    <code className='font-semibold'>{row[0]}</code>
                    <span className='text-muted-foreground'>{row[1]}</span>
                  </div>
                ))}
              </div>
              <p className='text-muted-foreground mt-5 text-sm'>
                {t(
                  'This page covers image generation only. Image editing, audio, video, batch generation, and higher resolutions are supported only when separately announced and listed by the service.'
                )}
              </p>
            </section>
          </div>
        </div>
      </main>
      <Footer />
    </PublicLayout>
  )
}
