# tutor — sources

> The external facts the repository's documents under `docs/` rest on: a provider's terms, a price, a law, a standard, a library's documentation. Here are their records — the address, the version or date of the page, the date it was read and a quote in the source's language — and [architecture.md](architecture.md) and the decision log hold the claim and a citation of the record by its number, as a section: `sources 1.1` (RULES.md, The bar; R143 spec).
>
> A record is a subsection "N.M" of a topic: one source, or one reading of several sources for one decision. A record's number never changes: a new one goes at the end of its topic, a new topic at the end of the file. A source read again is an item with its date in the same record, and the earlier text is not edited. The model is sources spec. The results of measurements are not here but in `docs/measurements.md`, which the first measurement starts.
>
> Until T132 these records stood in architecture 4, which stays as a pointer to them: the decision log, the archives of `docs/plan/` and spec cite it, and those are not rewritten (R102; T132). The quotes are verbatim; what is retold is ours, and our reading of terms or of a law is not a lawyer's. Not here: the quotes an entry of the decision log gives itself, which stay in the entry (RULES.md, How we work: a decision is never rewritten).

## 1. AI providers

### 1.1. Google Cloud: the terms of generative AI

Read 2026-10-20, unless said otherwise.
- [Service Specific Terms](https://cloud.google.com/terms/service-terms), "Last modified October 12, 2026", the section "20. Generative AI Services", the item "d. Age Restrictions" — §20(d) from here on: "Customer will not, and will not allow End Users to, use a Generative AI Service as part of a website, Customer Application, or other online service that is directed towards or is likely to be accessed by individuals under the age of 18." The text was taken from the page itself. On the same page: "Gemini Enterprise Agent Platform (formerly Vertex AI Platform)".
- The same page, on training: "Google will not use Customer Data to train or fine-tune any AI/ML models without Customer's prior permission or instruction."
- The same page, the item "f. Suspected Violations" — §20(f) from here on: "Google may immediately suspend or terminate Customer's use of a Generative AI Service based on any suspected violation of … subsection (d) above." A reference to another item of the same terms is left out. Checked 2026-10-21 on the page itself, "Last modified October 12, 2026".
- [The archive of the Service Specific Terms](https://cloud.google.com/legal/archive/terms/service-terms/index-20240718), "Last modified: July 18, 2024": the same item "d. Age Restrictions" in the same words. Checked 2026-10-21.
- [Zero data retention](https://docs.cloud.google.com/gemini-enterprise-agent-platform/resources/zero-data-retention):
  - "By default, Google's published Gemini models cache Customer Data"; the cache is deleted "no later than 24 hours after it's written" and is turned off on the project;
  - "Only customers whose use of Google Cloud is governed by the Google Cloud Platform Terms of Service are subject to prompt logging for abuse monitoring" — one can ask for an exception to it.
- [Abuse monitoring](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/abuse-monitoring): the requests the classifiers find are kept "for up to 90 days".

### 1.2. Google: models and prices, 2026-10-20

Read 2026-10-20. The measurement no longer takes them (R06, the 2026-10-21 amendment). They stay here as the basis of the former estimate, and the terms of sources 1.1 as the source of architecture 2.2 and R47 spec.
- [Model versions](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/model-versions) — release and retirement:
  - `gemini-4-argon` — October 13, 2026, retirement "October 13, 2027 or later";
  - `gemini-3.8-flash` — September 2, 2026, "September 2, 2027 or later";
  - `gemini-3.5-flash-lite` — July 21, 2026, "July 21, 2027 or later";
  - `gemini-3.6-flash` is retired on November 19, 2026, and `gemini-2.5-pro`, `gemini-2.5-flash` and `gemini-2.5-flash-lite` on October 20, 2026.
- [Pricing](https://cloud.google.com/gemini-enterprise-agent-platform/generative-ai/pricing), the global endpoint, per 1M tokens:
  - 3.8 Flash: "introductory pricing of $0.75 / $3.75 per 1M tokens input / output through December 31, 2026. Starting January 1, 2027, standard pricing of $1.5 / $7.5";
  - 3.5 Flash-Lite: $0.30 input, $2.50 output, "Text output (response and reasoning)";
  - 4 Argon: "through November 30, 2026" $2 input and $10 output, "Starting December 1, 2026" — $4 and $20;
  - the endpoints other than global cost about 10% more.
- [Batch inference](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/capabilities/batch-inference): "Batch processing is offered at a 50% discounted rate".
- Preview models, which the measurement does not take (R06), — for example [`gemini-3.1-pro-preview`](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/gemini/3-1-pro): "Launch stage: Public preview". In the card: "This product or feature is a Generative AI Preview offering, subject to the "Pre-GA Offerings Terms"". Previews have no retirement dates.
- [Image understanding](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/capabilities/image-understanding):
  - "The models might also hallucinate when interpreting handwritten text in images documents";
  - at Gemini 3 a picture is 280, 560, 1,120 or 2,240 tokens by `media_resolution`, 1,120 by default.
- [Content generation parameters](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/capabilities/content-generation-parameters): `seed` — "best effort … Deterministic output isn't guaranteed". In the cards of 3.6 Flash, 3.5 Flash-Lite and 4 Argon, custom values of `temperature` are "ignored".
- [API keys](https://docs.cloud.google.com/docs/authentication/api-keys): "Authorization keys are API keys that are bound to a service account"; "Only use authorization keys to accelerate initial exploration of Google Cloud".
- The Go SDK `google.golang.org/genai`: v1.75.0 of 2026-10-14, Apache-2.0 ([go-genai](https://github.com/googleapis/go-genai)).

### 1.3. Anthropic: models, prices and data, 2026-10-20

Read 2026-10-20, unless said otherwise.
- [Models overview](https://platform.claude.com/docs/en/models/overview) — every current model takes "text and image" as input:
  - `claude-haiku-5-5` — retirement "Not sooner than October 7, 2027";
  - `claude-opus-5-5` — "Not sooner than September 22, 2027";
  - `claude-fable-5-1` — "Not sooner than September 1, 2027".
- [Pricing](https://platform.claude.com/docs/en/about-claude/pricing), per 1M tokens: Haiku 5.5 — $0.10 input and $0.50 output (a request of up to 100,000 tokens); Opus 5.5 — $4 and $20; Fable 5.1 — $10 and $50. The prices of Haiku 5.5 and Opus 5.5 were checked again on 2026-10-26, before the development run of T27, and on 2026-10-28, before the runs of T73: the same. Sonnet 5.5 — $2 and $10, read 2026-10-28 for the extended measurement (T90), and the same day the prices of Haiku 5.5 and Opus 5.5 were the same. The prices of Haiku 5.5 and Opus 5.5 were checked again on 2026-10-29, before the runs of T99: the same. Thinking is paid as output; batch — "a 50% discount".
- [Vision](https://platform.claude.com/docs/en/build-with-claude/vision): a picture costs "⌈width / 28⌉ × ⌈height / 28⌉ visual tokens".
- [Data](https://platform.claude.com/docs/en/manage-claude/api-and-data-retention): Fable 5.1 requires 30 days of retention and is "not available under ZDR unless expressly authorized by Anthropic". That is why the measurement's strong model is Opus 5.5 (R06).
- [Minors](https://support.claude.com/en/articles/9307344-responsible-use-of-anthropic-s-models-guidelines-for-organizations-serving-minors): the article of 16.03.2026 has not changed since privacy 5.3 spec. The new [Usage Policy](https://www.anthropic.com/legal/aup) was published in advance and takes effect on 12.11.2026 ("Effective November 12, 2026"); the version of 15.09.2025 is in force now. The new one adds: "Any specific age eligibility requirements for use of our products will be set out in our Terms of Service."
- [Workspaces](https://platform.claude.com/docs/en/manage-claude/workspaces): "Spend limits: Cap monthly spending for a workspace"; "You cannot set limits on the Default Workspace".
- [Thinking](https://platform.claude.com/docs/en/build-with-claude/thinking): custom values of `temperature` give an error 400 on the current models; the [Messages API](https://platform.claude.com/docs/en/api/messages/create) has no `seed`.
- The middle model, which the measurement does not take (R06), — `claude-sonnet-5-5`: $2 input and $10 output per 1M tokens, retirement "Not sooner than September 28, 2027" (the same pages of prices and models).
- The Go SDK `github.com/anthropics/anthropic-sdk-go`: v1.82.0 of 2026-10-15, MIT.

### 1.4. Anthropic: images in a request

Read 2026-10-21 and 2026-10-26.
- [Vision](https://platform.claude.com/docs/en/build-with-claude/vision), checked 2026-10-21. Images in one request: "600 per request on the API, for all other models", and at the models with a window of 200k, "100 per request on the API". The window of Haiku 5.5 and Opus 5.5 is "1M tokens" ([Models overview](https://platform.claude.com/docs/en/about-claude/models/overview)), so they get 600. With more than 20 images, each is at most 2,000 px: "resize each image so that neither dimension exceeds 2000 px, or keep the request to 20 or fewer image and document blocks". A picture of 1,000×1,000 px the model does not shrink: in the table of resizing both tiers have "Not resized".
- [Vision](https://platform.claude.com/docs/en/build-with-claude/vision), read 2026-10-26, — several images in a request (T16):
  - "You can include multiple images in a single request, and Claude analyzes them jointly. This is useful for comparing images, asking about differences, or working with a sequence such as pages of a document.";
  - "When sending several images, introduce each one with a short text label (Image 1:, Image 2:, and so on) so you can refer to them by name in your prompt and in follow-up turns.";
  - the limit is "100 for each request on the API, for models with a 200k-token context window" and "600 for each request on the API, for all other models"; "If a single API request contains more than 20 images, a stricter per-image dimension limit applies to every image in that request." A request of the measurement has at most three images: the final one and two frames.

### 1.5. Anthropic: prompt caching, 2026-10-25

[Prompt caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching), checked 2026-10-25 against the text of the page (T41):
- "By default, the cache has a 5-minute lifetime. The cache is refreshed for no additional cost each time the cached content is used."; "If you find that 5 minutes is too short, Anthropic also offers a 1-hour cache duration at additional cost";
- "Cache reads: When cached content is used (10% of base input token price, or 2.5% on Claude Fable 5.1 and Claude Mythos 5.1, and 5% on Claude Opus 5.5 and Claude Sonnet 5.5)";
- "the minimum cacheable prompt length is: 512 tokens for Claude Fable 5.1, … Claude Opus 5.5, … and Claude Haiku 5.5". The other models of the list are left out.

### 1.6. Anthropic: latency and speed, 2026-10-25

Read 2026-10-25 (T42). The pages show no date of their own.
- [Reducing latency](https://platform.claude.com/docs/en/test-and-evaluate/strengthen-guardrails/reduce-latency): the levers are "1. Choose the right model", "2. Optimize prompt and output length", "3. Stream responses". "For speed-critical applications, Claude Haiku 5.5 offers the fastest response times while maintaining high intelligence. Effort is its main control for speed and cost."; "The fewer tokens the model has to process and generate, the faster the response will be." On streaming: "This can significantly improve the perceived responsiveness of your application".
- [Models overview](https://platform.claude.com/docs/en/models/overview), the row "Comparative latency": Fable 5.1 has "Slower", Opus 5.5 "Moderate", Sonnet 5.5 "Fast", Haiku 5.5 "Fastest". "Actual latency depends on prompt length, output length, and thinking effort."
- [Fast mode](https://platform.claude.com/docs/en/build-with-claude/fast-mode):
  - "Speed benefits are focused on output tokens per second (OTPS), not time to first token (TTFT)";
  - the price is "Claude Opus 5.5 | $8 USD / MTok | $40 USD / MTok", the cells of the table's row separated here by " | ". The page [Optimizing for cost and intelligence](https://platform.claude.com/docs/en/about-claude/models/optimizing-for-cost-and-intelligence) says of Fast mode: "at twice the standard token prices";
  - "Fast mode is not available with the Batch API."; "Requests at different speeds do not share cached prefixes.";
  - ZDR is in the page's header: "eligible", with the reservation "Excludes Covered Models".
- [Service tiers](https://platform.claude.com/docs/en/api/service-tiers): "Priority Tier capacity commitments are no longer available for purchase."; "Priority Tier is supported on all available Claude models except Claude Fable 5.1, Claude Haiku 5.5, …" — Sonnet 5.5 and Opus 5.5 are in the same list of exceptions.
- [Effort](https://platform.claude.com/docs/en/build-with-claude/effort): "Claude Opus 5.5 and Claude Haiku 5.5 default to medium."; "Effort is a behavioral signal, not a strict token budget." On the level `low`, the page [Thinking: steering and cost](https://platform.claude.com/docs/en/build-with-claude/thinking-steering-and-cost): "Skips thinking for simple tasks where speed matters most."
- [Thinking](https://platform.claude.com/docs/en/build-with-claude/thinking):
  - Haiku 5.5 has "Thinking off at high effort or below", and Opus 5.5 "Thinking can't be turned off on these models.";
  - "the tokens Claude spends reasoning are billed as output tokens, even when the thinking text isn't returned to you, and they count toward max_tokens alongside the response text".

  On Haiku 5.5, the page [Prompting Claude Haiku 5.5](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/prompting-claude-haiku-5-5): "sometimes writes reasoning-like text in the reply that users see. This occurs more often with thinking off or at low effort."
- [Prompt caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching), beyond the quotes of sources 1.5:
  - "Cache write tokens with a 5-minute TTL are 1.25 times the base input tokens price.";
  - the automatic cache "applies the cache breakpoint to the last cacheable block";
  - "You generally see improved time-to-first-token for long documents.";
  - "Changing the output_config.effort value always invalidates message blocks"; "Switching between speed: "fast" and standard speed invalidates system and message caches";
  - "Prompt caching (both automatic and explicit) is ZDR eligible."
- [Structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs): "The injected prompt costs you tokens like any other system prompt."; "The first time you use a specific schema, there is additional latency while the grammar compiles."; "Compiled grammars are cached for 24 hours from last use"; "properties in objects maintain their defined ordering from your schema", with the reservation "required properties appear first, followed by optional properties". In the retention table of [API and data retention](https://platform.claude.com/docs/en/manage-claude/api-and-data-retention), structured outputs have "Yes (qualified)": "Your prompts and Claude's outputs are not stored. Only the JSON schema is cached, for up to 24 hours since last use."
- [Batch processing](https://platform.claude.com/docs/en/build-with-claude/batch-processing):
  - "most batches finishing in less than 1 hour while reducing costs by 50%"; "Batches expire if processing does not complete within 24 hours.";
  - Opus 5.5 has "$2 / MTok | $10 / MTok", input and output;
  - "Batch processing stores request and response data for up to 29 days after batch creation. You can delete a message batch at any time after processing".

  In [API and data retention](https://platform.claude.com/docs/en/manage-claude/api-and-data-retention), Batch has "29-day retention; async storage required."
- [Vision](https://platform.claude.com/docs/en/build-with-claude/vision): "To minimize latency and to simplify coordinate-based workflows, prefer resizing images before uploading them."
- [Data residency](https://platform.claude.com/docs/en/manage-claude/data-residency): the region `us` is "priced at 1.1x the standard rate across all token pricing categories".
- [Messages API](https://platform.claude.com/docs/en/api/messages/create): `usage` has `output_tokens_details.thinking_tokens`, `service_tier` and `speed`, and no fields of time.
- **Read again 2026-11-13** (T140; Д12): the gain of Fast mode, which architecture 3.12 gave without a quote. [Fast mode](https://platform.claude.com/docs/en/build-with-claude/fast-mode), by its `.md` version: "Fast mode delivers up to 2.5x higher output tokens per second at premium pricing."; in "How fast mode works": "Up to 2.5x higher output tokens per second compared to standard speed", and the quote on OTPS and TTFT above stands. The models have changed since: "Fast mode is available on Claude Fable 5.2, Claude Mythos 5.2, and Claude Opus 5.5. Fast mode is in research preview on Claude Opus 5 and deprecated on Claude Opus 4.8." The page's header is as above: ZDR "eligible", with "Excludes Covered Models", and whether Fable 5.2 and Mythos 5.2 are Covered Models was not read. The copies of the Wayback Machine of [2026-10-23 06:28 UTC](http://web.archive.org/web/20261023062820/https://platform.claude.com/docs/en/build-with-claude/fast-mode) and [2026-10-26 23:59 UTC](http://web.archive.org/web/20261026235916/https://platform.claude.com/docs/en/build-with-claude/fast-mode), on either side of the reading of T42, have the same "Up to 2.5x higher output tokens per second compared to standard speed" and "Fast mode is available on Claude Opus 5.5 and in research preview on Claude Opus 5 and Claude Opus 4.8."

### 1.7. Anthropic: marking, thinking and the board's schema, 2026-10-25

Read 2026-10-25 (T45). The pages show no date of their own.
- [Coordinates and bounding boxes](https://platform.claude.com/docs/en/build-with-claude/vision-coordinates):
  - "Claude works best with absolute pixel coordinates."; a box is "(top-left and bottom-right corners) in pixel coordinates"; "Claude does not work well when you ask for normalized coordinates";
  - "Coordinate accuracy is best when you state the expected coordinate format in your prompt and spot-check results visually before processing at scale.";
  - the model pads the picture: "Claude then pads every image, resized or not, up to the next multiple of 28 pixels on the bottom and right edges";
  - "Clamp returned coordinates to the resized dimensions before rescaling, so a point slightly outside the image can't map outside your original.";
  - a request whose picture, with `"oversized_image": "error"`, "would be resized is rejected with a 400" — with the error `invalid_request_error`.
- [Vision](https://platform.claude.com/docs/en/build-with-claude/vision): "Claude's coordinate and localization outputs are approximate."
- [Thinking](https://platform.claude.com/docs/en/build-with-claude/thinking):
  - in the table of settings by model, Haiku 5.5 with `"disabled"` has "Thinking off at `high` effort or below", and Sonnet 5.5 with `"between_tools"` has "Up-front thinking off at `high` effort or below";
  - on Haiku 5.5: "At `xhigh` or `max` effort, that combination returns a 400 error.";
  - "Claude Sonnet 5.5 also has thinking on by default, and it rejects `thinking: {type: "disabled"}` with a 400 error. To turn off up-front thinking, send `thinking: {type: "between_tools"}` instead."; "Without tools, the response contains only text";
  - on `between_tools`: "At `xhigh` or `max`, a request with it returns a 400 error."
- [Structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs): in the list of what is supported, "`anyOf` and `allOf` (with limitations: `allOf` with `$ref` not supported)", and in the list of what is not, "Numerical constraints (such as `minimum`, `maximum`, `multipleOf`)".

### 1.8. OpenAI: models, prices and data, 2026-10-20

Read 2026-10-20, unless said otherwise.
- [Models](https://developers.openai.com/api/docs/models): "All latest OpenAI models support text and image input". `gpt-6-luna` is "Our most efficient model", `gpt-6-astra` "Our most capable model". They have [no](https://developers.openai.com/api/docs/deprecations) retirement dates.
- [Pricing](https://developers.openai.com/api/docs/pricing), per 1M tokens: `gpt-6-luna` — $0.10 input and $0.50 output; `gpt-6-astra` — $10.00 and $50.00. "Batch and Flex are priced at 50% of Standard rates". The page openai.com/api/pricing did not open (403).
- [Images](https://developers.openai.com/api/docs/guides/images-vision): "patch_count = ceil(width/32)×ceil(height/32)", and `gpt-6-astra` has the multiplier 1.2. The rule for `gpt-6-luna` is not published.
- The same page, checked 2026-10-21:
  - in the table of multipliers `gpt-6-astra` and `gpt-5.6-luna` have 1.2, and `gpt-6-luna` is neither in it nor anywhere on the page;
  - images in one request: "Up to 1,500 images per request";
  - `gpt-6-astra` at `high` has "up to 2,500 patches and a 65,535-pixel maximum dimension". The measurement's crop, up to ~600×450 px, that is up to ~285 patches, comes to the model unshrunk;
  - the detail by default: "If you omit the parameter, it defaults to `auto` in both the Responses API and the Chat Completions API"; `auto` is "Use the model's default sizing behavior, shown in the model sizing table". `gpt-6-luna` is not in this table.
- [Your data](https://developers.openai.com/api/docs/guides/your-data):
  - "data sent to the OpenAI API is not used to train or improve OpenAI models (unless you explicitly opt in…)";
  - "abuse monitoring logs are generated for all API feature usage and retained for up to 30 days";
  - zero data retention is "subject to prior approval by OpenAI".
- The [Under-18 guidance](https://developers.openai.com/api/docs/guides/safety-checks/under-18-api-guidance) has not changed since privacy 5.3 spec. In the [Services Agreement](https://cdn.openai.com/osa/openai-services-agreement.pdf), the PDF "ONLINE v.010126", §3.3(c), the Customer will not "allow minors to use OpenAI Services without consent from their parent or guardian".
- [Spend limits](https://developers.openai.com/api/docs/guides/spend-limits): "Enforce a hard limit" on a project.
- [Latest model](https://developers.openai.com/api/docs/guides/latest-model): "When reasoning effort is not `none`, remove `temperature`, `top_p`, and `top_logprobs`"; "GPT-6 Astra and GPT-6.1 Sol do not support `none`". In [Chat](https://developers.openai.com/api/reference/resources/chat), `seed` is "in Beta … best effort … Determinism is not guaranteed", and the Responses API has none.
- The middle model, which the measurement does not take (R06), — `gpt-6.1-sol`: "to balance intelligence and cost", $2.00 input and $10.00 output per 1M tokens (the same pages of models and prices).
- The Go SDK `github.com/openai/openai-go/v3`: v3.82.0 of 2026-10-18, Apache-2.0.

### 1.9. OpenAI: speed and price, 2026-10-25

The sources of architecture 3.12 (T44). Read 2026-10-25, by the `.md` versions of the pages.
- The quotes of sources 1.8 were checked anew, and all are in place. "Batch and Flex are priced at 50% of Standard rates" now stands not on the pricing page but on the pages of the models `gpt-6-luna` and `gpt-6-astra`.
- [GPT-6 Luna](https://developers.openai.com/api/docs/models/gpt-6-luna): "`reasoning.effort` supports `none`, `low`, `medium` (default), `high`, `xhigh`, and `max`." For `none`, [Reasoning](https://developers.openai.com/api/docs/guides/reasoning) names "Latency-critical tasks that do not benefit from any reasoning or multi-chained tool calls." OpenAI publishes no numbers of time and tokens for `none`.
- [Fast mode](https://developers.openai.com/api/docs/guides/fast-mode):
  - "Priority processing was renamed Fast mode on July 30, 2026.";
  - "Fast mode delivers up to 2.5× faster speeds and more consistent latency with pay-as-you-go pricing." And the [pricing page](https://developers.openai.com/api/docs/pricing) ties this number to one model: "In Fast mode, GPT-5.6 Sol runs up to 2.5× faster than Standard processing.";
  - "Fast mode for GPT-6 Astra does not include a latency SLA.";
  - "If your traffic ramps too fast, the system may downgrade some Fast mode requests to standard speeds and charge standard rates.";
  - on the pages of the models: "Fast mode is priced at 2x the applicable rates.";
  - in the same guide: "the speed at which Fast mode operates for `gpt-5.6-sol` to make it up to 2.5×" and "Fast mode is compatible with data residency, Zero Data Retention, and a Business Associate Agreement (BAA), subject to model-specific availability."
- [Ultrafast](https://developers.openai.com/api/docs/guides/ultrafast-mode): "It is broadly available for GPT-6 Astra and GPT-6.1 Sol." In it Astra costs $60 and $300 per 1M tokens, by the pricing page.
- [Flex](https://developers.openai.com/api/docs/guides/flex-processing): "It's ideal for non-production or lower priority tasks, such as model evaluations, data enrichment, and asynchronous workloads."; "Flex processing is in beta with limited model availability."
- Data residency:
  - on the pricing page: "Regional processing (data residency) endpoints are charged a 10% uplift for models released on or after March 5, 2026.";
  - in [Your data](https://developers.openai.com/api/docs/guides/your-data): "To use data residency with any region other than the United States, you must be approved for abuse monitoring controls, and execute a Modified Retention amendment." Images in the EU region: "Image support in these regions requires approval for enhanced Zero Data Retention or enhanced Modified Abuse Monitoring."
- [Prompt caching](https://developers.openai.com/api/docs/guides/prompt-caching): "The minimum cacheable prompt length is 1,024 tokens for GPT-5.6 and later"; "For GPT-5.6 and later, cache writes cost 1.25× the standard, uncached input-token rate for the default 30-minute lifetime." The board's common beginning is about 500 tokens, below the minimum.
- A picture of 1,000 × 600 at `gpt-6-astra`, by the formula of patches of sources 1.8, is the executor's estimate. At `auto`, `high` and `original` it is 608 patches and 730 tokens, and at `low` 192 tokens. For `gpt-6-luna` the rule is not published: it is neither in the table of multipliers nor in the calculator.

### 1.10. Fast inference providers, 2026-10-25

The sources of architecture 3.12 (T44). Read 2026-10-25. The terms are our reading, not a lawyer's: each question that would stop a provider goes to a lawyer before the provider is called.
- **Groq:**
  - [models](https://console.groq.com/docs/models): "Preview models are intended for evaluation purposes only and should not be used in production environments as they may be discontinued at short notice." Both models with images are in the table Preview, and `qwen/qwen3.8-27b` has 450 tokens a second there;
  - [images](https://console.groq.com/docs/vision): "Each image counts as 2048 input tokens."; the price of `qwen/qwen3.8-27b` is $0.80 and $4.00 per 1M tokens, by the table of models;
  - [Services Agreement](https://console.groq.com/docs/legal/services-agreement), "Last Modified: June 22, 2026":
    - §4.2: "For clarity, Groq is not permitted to use Inputs or Outputs for training or fine-tuning any AI Model Services or other models, unless explicitly granted permission or instructed by Customer.";
    - §6.3: "Customer will be solely responsible for ensuring that the Customer Application complies with all applicable laws and regulations", when the application is "directed towards or is likely to be accessed by individuals under the age of majority";
    - §3.6: "Except as otherwise expressly indicated in Documentation, Groq’s DPA and BAA do not apply to Beta Services." and "Beta Services are not Cloud Services under this Agreement.";
  - [data](https://console.groq.com/docs/your-data): "By default, Groq does not retain customer data for inference requests."; "All customers may enable Zero Data Retention (ZDR) in Data Controls settings."; "All customer data is retained in Google Cloud Platform (GCP) buckets located in the United States.";
  - the license of the model Muse Spark Open, on [its page](https://huggingface.co/meta-models/Muse-Spark-Open-816B-A42B): "Muse Spark Open is not intended for individuals under the age of 18." And [Qwen3.8-27B](https://huggingface.co/api/models/Qwen/Qwen3.8-27B) has `"license":"apache-2.0"`.
- **Together AI:**
  - [privacy and security](https://docs.together.ai/docs/privacy-and-security): "Data sharing for training other models is opt-in and not enabled by default."; "By default, Together stores the prompts you send and the responses models return, and may use them for product improvements.";
  - [ZDR](https://docs.together.ai/docs/zero-data-retention): "ZDR is not enabled by default.";
  - [the terms](https://www.together.ai/terms-of-service), "Updated May 19, 2026": "You will not use the Services to transmit or provide to the Company any financial or medical information of any nature or any sensitive personal data"; and one more prohibition: "engage in competitive analysis or benchmarking";
  - [the privacy policy](https://www.together.ai/privacy): "Our Services are not directed to, or intended for anyone under the age of thirteen (13).";
  - the [table of models with images](https://docs.together.ai/docs/serverless/models) has four models, among them Qwen3.5 9B at $0.17 and $0.25 per 1M tokens;
  - [images](https://docs.together.ai/docs/inference/vision/overview): "Each image breaks into a tile grid (capped at 2×2 of 560-pixel tiles) and you pay 1,601 tokens per tile." The table and the formula of the same page disagree for sides from 560 to 1,119 px, so the board's picture is 1,601 or 6,404 tokens.
- **SiliconFlow** — its API takes a picture, by the [documentation](https://docs.siliconflow.com/en/userguide/capabilities/vision): "For VLM models, you can invoke the `/chat/completions` API by constructing a `message` containing either an `image URL` or a `base64-encoded image`." [The terms](https://docs.siliconflow.com/en/legals/terms-of-service), "Update Date: Sep. 09th, 2026": "you will not upload or provide, directly or indirectly through this service, any personal information of children under the age of 14". The [terms of the Chinese service](https://api-docs.siliconflow.cn/docs/legals/terms-of-service) say the same: "您不会上传或通过本服务直接或通过其他方式提供14岁以下儿童的任何个人信息". Our translation: "you will not upload or pass through this service, directly or otherwise, any personal information of children under 14".
- **Cerebras:**
  - [images](https://inference-docs.cerebras.ai/capabilities/image-inputs): "Image inputs are available for `qwen-3.8-27b` with Shared Inference."; "This feature is in Public Preview"; "Image inputs are processed as soon as they are received, and the original image payloads are not persisted."; an image token is "32 × 32 pixels per image token";
  - [preview releases](https://inference-docs.cerebras.ai/support/preview-releases): "Public Preview features are available to all customers and represent a stable, production-ready implementation.";
  - [the model](https://inference-docs.cerebras.ai/models/qwen-3.8-27b): "Pricing: $0.99 per million input tokens and $1.49 per million output tokens." The [pricing page](https://www.cerebras.ai/pricing) has "~1,850 tokens/s";
  - [the terms](https://www.cerebras.ai/terms-of-service): "For clarity, the foregoing does not grant Cerebras the right to use Service Content for the purpose of training or fine-tuning models." The same terms prohibit use "for benchmarking or competitive analysis of the Service";
  - [the help page](https://www.cerebras.ai/policies): "No – we do not store, leverage, or share any inputs or outputs from our AI services." On children, the same page: "No – our products are not intended for minors and we do not knowingly collect their personal data." "If we discover such data, we delete it.";
  - [the privacy policy](https://www.cerebras.ai/privacy-policy): "We do not retain inputs and outputs associated with our training, inference and chatbot Services." But the policy itself "does not apply to Personal Data we process in our role as a data processor or service provider on behalf of our customers and partners", and the DPA is a PDF whose text did not extract.
- **SambaNova:**
  - [models](https://docs.sambanova.ai/docs/en/models/sambacloud-models): "Preview models are intended for evaluation purposes and developer experimentation only, and should not be used in production environments."; `gemma-4-31B-it` is the only model with images, and it is in Preview. Its price is $0.38 and $1.15 per 1M tokens, by the [pricing page](https://cloud.sambanova.ai/plans/pricing). Gemma 4 has `license: apache-2.0` in its [card](https://huggingface.co/google/gemma-4-31b-it);
  - [the DPA](https://sambanova.ai/data-processing-agreement-sambanova): "shall not use Customer Personal Data to train or fine-tune any machine-learning or artificial-intelligence model"; requests and responses "are retained for no more than 30 days solely for security, fraud and abuse monitoring and then deleted";
  - [the terms](https://sambanova.ai/cloud-end-user-license-agreement), §1(g): without consent one may not "publish or disclose information or results relating to capabilities, performance, performance comparisons or other “benchmarking” activities relating to the Service or our infrastructure".
- **Fireworks** — its [library of models](https://fireworks.ai/models?filter=Vision) has nine serverless models with images, Kimi K3 among them. [The terms](https://fireworks.ai/terms-of-service) are a PDF, "Last Updated: July 10, 2026". The executor extracted its text:
  - §3.3: "(vii) your Input does not include any personal information of children under 13 (or the applicable age of digital consent).";
  - §3.6: "We will not use your Content to train our own models or to improve the Service.";
  - §2.2(i) prohibits use "for benchmarking or competitive analysis of the Service".
- **Mathpix** — its price and time are already in sources 2.1:
  - [OCR](https://mathpix.com/ocr): "OCR individual images of handwritten and printed equations, text, tables, and diagrams to get digital formats like LaTeX, AsciiMath, and MathML.";
  - on the [page of languages](https://mathpix.com/language-support), the handwritten ones are Hindi, English and the languages in Latin script, and Russian is only among the printed ones;
  - [privacy in the API](https://docs.mathpix.com/concepts/privacy): "Images and results are accessible to the Mathpix QA team for quality improvements"; "Mathpix can also pin improve_mathpix at the account level on your app key (for example, as part of an enterprise agreement).";
  - the DPA and the EU: "EU residency commitments can be incorporated into your Data Processing Agreement." ([documentation](https://docs.mathpix.com/concepts/eu-data-residency)) and "API customers may additionally choose our EU endpoint (eu.api.mathpix.com)" ([the privacy policy](https://mathpix.com/privacy));
  - retention: the [privacy policy](https://mathpix.com/privacy) has "We will retain Customer Provided and Automatically Collected data for up to 30 days, unless you have opted out of data retention for QA.", and the [documentation](https://docs.mathpix.com/concepts/data-retention) has, for a picture, "Retained for up to 90 days".

### 1.11. Minors: the terms of OpenAI and Anthropic, 2026-11-01

The sources of architecture 3.6 (T02). Read 2026-11-01. The quotes are verbatim; what is retold is ours, not a lawyer's.
- **OpenAI:**
  - [Under-18 guidance](https://developers.openai.com/api/docs/guides/safety-checks/under-18-api-guidance), the `.md` version of the same page:
    - "Organizations serving minors must comply with all applicable child protection, safety, and privacy laws, including the Children's Online Privacy Protection Act (COPPA).";
    - "You should not use OpenAI services to process any personal data of children under 13 or the applicable age of digital consent without first implementing zero data retention in our API.";
    - "Organizations serving minors must take reasonable steps to ensure that the content they are serving to minors via the API is safe and age appropriate", and the measures, which "may include, but is not limited to": "1. Providing age-appropriate disclosures to minors about AI tools and how to use them responsibly. 2. Implementing age-appropriate content filters to address potentially sensitive content. 3. Implementing reasonable monitoring and reporting mechanisms, including escalation paths for high-risk interactions. 4. Where required or otherwise appropriate for your use case, using age assurance systems to ensure only intended users can access the product.";
    - "Use OpenAI’s most current flagship models, which incorporate the latest safety protections, particularly when building experiences for minors.";
    - "OpenAI reserves the right to audit organizations for compliance with this policy."

    privacy 5.3 spec, checked 2026-10-18, names filters by age, monitoring and disclosure, and has neither age assurance nor the flagship models;
  - [Services Agreement](https://openai.com/policies/services-agreement/), "Updated: December 1, 2025", "Effective: January 1, 2026", para. 3.3: "Customer will not, and will not permit End Users to: … (c) allow minors to use OpenAI Services without consent from their parent or guardian". Items (a) and (b) are left out. The page itself answers a request of `curl` with 403, and the text was taken from a copy of the Wayback Machine of 2026-10-28;
  - [Usage Policies](https://openai.com/policies/usage-policies/), "Effective: November 20, 2026", and in the history of changes, "2026-10-21: … We’ve also clarified protections for children and teens". The beginning of the entry is left out. The section "Keep young people safe": "OpenAI tools must not be used to endanger anyone under 18." and "We prohibit use of our services for:", and the first item of the list is "exposing minors to age-inappropriate content, such as graphic self-harm, sexual, or violent content". This page answers 403 too, and the text was taken from a copy of the Wayback Machine of 2026-11-01. The same prohibition "exposing minors to age-inappropriate content" stands in the version now in force, "Effective: October 29, 2025", in the section "Keep minors safe" (a copy of the Wayback Machine of 2026-09-30);
  - [Your data](https://developers.openai.com/api/docs/guides/your-data):
    - the abuse logs "may contain certain customer content, such as prompts and responses, as well as metadata derived from that customer content, such as classifier outputs", and "By default, abuse monitoring logs are generated for all API feature usage and retained for up to 30 days";
    - zero data retention and Modified Abuse Monitoring keep "customer content" out of these logs and "are subject to prior approval by OpenAI and acceptance of additional requirements";
    - "Customers who enable Modified Abuse Monitoring or Zero Data Retention are responsible for ensuring their users abide by OpenAI's policies for safe and responsible use of AI and complying with any moderation and reporting requirements under applicable law.";
    - "Image and file inputs are scanned for CSAM content upon submission. If the classifier detects potential CSAM content, the image will be retained for manual review, even if Zero Data Retention, Modified Abuse Monitoring, or Private Retention with PSP is enabled.";
  - [ZDR with Private Safety Processing](https://developers.openai.com/api/docs/guides/private-safety-processing): "enables offline, automated safety review without OpenAI retaining customer prompts or responses", but "Customer content is stored in customer-controlled storage", and the records are written "to your regional cloud storage with a TTL of 30 days";
  - [Safety checks](https://developers.openai.com/api/docs/guides/safety-checks): "Safety identifiers are recommended but not required." By [Safety best practices](https://developers.openai.com/api/docs/guides/safety-best-practices), a safety identifier is "a string that uniquely identifies each user";
  - [Moderation](https://developers.openai.com/api/docs/guides/moderation): "The moderation endpoint is free to use". The scores can also be had in the request of generation itself, "pass a top-level `moderation` object in the generation request", and the page names no price for that.
- **Anthropic:**
  - [Guidelines for organizations serving minors](https://support.claude.com/en/articles/9307344-responsible-use-of-anthropic-s-models-guidelines-for-organizations-serving-minors), `dateModified` 2026-03-16, — the same article as in privacy 5.3 spec. The help site is Intercom, and the text was taken from the page:
    - the measures, which "may include, but are not limited to": "Age verification systems to ensure only intended users can access the product", "Content moderation and filtering to block inappropriate or harmful content", "Monitoring and reporting mechanisms to identify and address potential issues", "Educational resources and guidance for minors on safe and responsible use of the product";
    - "we may provide a child-safety system prompt, which organizations serving minors should implement as part of a comprehensive suite of safety measures";
    - on COPPA: "Compliance with these regulations should be clearly stated on the organization’s website or similar public-facing documentation.";
    - "Organizations must disclose to their users that they are interacting with an AI system rather than a human.";
    - "Anthropic will periodically audit organizations for compliance with these safeguards.";
  - [Child Safety Guidance for Developers](https://support.claude.com/en/articles/15591275-child-safety-guidance-for-developers), `dateModified` 2026-06-26, — the Usage Policy links to it. On the article for minors: "which sets out the additional safeguards Anthropic requires for that audience". On its own oversight: "We monitor API usage for violations of the Usage Policy and take enforcement action consistent with our policies and legal obligations." It has no child system prompt either;
  - Anthropic has no separate API of moderation, by our reading of the index of the documentation: it has only the guide [Content moderation](https://platform.claude.com/docs/en/about-claude/use-case-guides/content-moderation), and that is about "how Claude can be used to moderate content within your digital application", that is, about moderation by a call of the model itself;
  - no published child system prompt was found: it is neither in the index of the documentation, [platform.claude.com/llms.txt](https://platform.claude.com/llms.txt), nor in the article for developers above;
  - [Usage Policy](https://www.anthropic.com/legal/aup), "Effective Nov 12, 2026": "Products serving minors, including organizations providing minors with the ability to directly interact with products that incorporate our API(s), must comply with the additional guidelines outlined in our Support article and all applicable laws." and "Any specific age eligibility requirements for use of our products will be set out in our Terms of Service." The requirement is not new: the version "Effective September 15, 2025", in force until 12.11.2026, says the same, "must comply with the additional guidelines outlined in our Help Center article" (a copy of the Wayback Machine of 2026-10-01). In the new version too: "All consumer-facing chatbots or AI agents that communicate directly with external users must disclose to users that they are interacting with AI rather than a human. This disclosure must be provided at a minimum at the beginning of each chat session or in the product interface.";
  - [Commercial Terms](https://www.anthropic.com/legal/commercial-terms), "Effective Jun 17, 2025", say nothing of age and minors: a search for minor, child, age and 18 finds nothing of the kind. But para. D.2 makes the Usage Policy part of the agreement: "each of which is incorporated by reference into these Terms";
  - [API and data retention](https://platform.claude.com/docs/en/manage-claude/api-and-data-retention): zero data retention — "To request ZDR for your organization, contact the Anthropic sales team.", and "Even with ZDR or HIPAA arrangements in place, Anthropic may retain data where required by law or where it has been flagged by Anthropic’s automated trust and safety systems. As a result, if a chat or session is flagged, Anthropic may retain inputs and outputs for up to 2 years."

### 1.12. The own key: spend limits and refusals of the providers, 2026-11-01

The sources of architecture 3.7 (T03). Read 2026-11-01, by the `.md` versions of the same pages; the pages show no date of their own. The quotes are verbatim; what is retold is ours.
- **Anthropic:**
  - [Rate limits](https://platform.claude.com/docs/en/api/rate-limits):
    - spend limits "set a maximum monthly cost an organization can incur for API usage";
    - "Each of the Start, Build, and Scale tiers carries a monthly spend cap, which is the maximum your organization can spend on the API each calendar month." The cap of a tier is $500, $1,000 and $200,000 a month. And new organizations "may start in the Evaluation tier, with limits below the standard limits shown on this page";
    - "You can also set your own spend limit below your tier’s cap to control costs";
    - "When usage reaches a spend limit you set, requests return HTTP 400 with error type `invalid_request_error`. The message starts with `You have reached your specified API usage limits`, or `You have reached your specified workspace API usage limits` for a workspace limit, and states when access resumes.";
    - the cap of a tier: "Once you reach your tier’s spend cap, API usage pauses until 00:00 UTC on the first day of the next month, unless you request a higher limit sooner." The response is HTTP 429 `rate_limit_error`, and "On the Messages API, `error.details.error_code` is `enforced_spend_limit_reached`. Use it to tell this response apart from a rate limit." And more: "the response has no `retry-after` header. Retrying, including the SDK’s automatic retries, fails until access resumes.";
    - "To protect Workspaces in your Organization from potential overuse, you can set custom spend and rate limits for each Workspace." and "You can’t set spend limits on the default Workspace."
  - [Workspaces](https://platform.claude.com/docs/en/manage-claude/workspaces):
    - the spend limit of a workspace: "Cap monthly spending for a workspace.";
    - "Only organization admins can create workspaces.";
    - a key can belong to one workspace: "API keys can be scoped to a single workspace." And on the keys without one: "Other API keys aren’t scoped to a workspace. With one of these keys, send a workspace ID header on each request to choose the workspace.";
    - on the workspace of Claude Code: "It is the only workspace that supports per-user monthly spend limits." and "You cannot create keys in it manually from the Console."
  - [API credits for subscribers](https://platform.claude.com/docs/en/about-claude/api-credits-for-subscribers) — the page on the API credits that a subscription to Claude gives:
    - "Every API key and workspace in the linked organization draws from the same balance. To cap spending on a project, give it its own workspace and set that workspace’s spend limit.";
    - "When the balance is exhausted, requests return:" and the text "Your credit balance is too low to access the Anthropic API. Please go to Plans & Billing to upgrade or purchase credits." The page does not name the status of the response.
  - [Errors](https://platform.claude.com/docs/en/api/errors):
    - "The API also returns a 400 when usage reaches an organization or workspace spend limit you set";
    - "402 - `billing_error`: There’s an issue with your billing or payment information.";
    - "The official SDK automatically retries transient failures (such as connection errors, rate limits, and 5xx server errors) with exponential backoff, twice by default, honoring the `retry-after` header when present."
  - [Messages](https://platform.claude.com/docs/en/api/messages/create), the object `usage`:
    - "Total input tokens in a request is the summation of `input_tokens`, `cache_creation_input_tokens`, and `cache_read_input_tokens`.";
    - "`output_tokens` remains the inclusive, authoritative total used for billing." The tokens of thinking, `thinking_tokens`, are "Always ≤ `output_tokens`".
  - [Spend Limits API](https://platform.claude.com/docs/en/manage-claude/spend-limits-api): "Scoped Admin API key required", and "Your organization must be on a Claude Enterprise plan."
  - [Usage and Cost API](https://platform.claude.com/docs/en/manage-claude/usage-cost-api): "Admin API credentials required." A key of the Admin API will do, or "a personal or service account key that isn’t scoped to a workspace; workspace API keys don’t work". And more: "The Admin API is unavailable for individual accounts."
  - [Pricing](https://platform.claude.com/docs/en/about-claude/pricing): the prices of the four models of the adapter Claude are the same as in its code (architecture 3.2): Haiku 5.5 — $0.10 input and $0.50 output, Sonnet 5.5 — $2 and $10, Opus 5.5 — $4 and $20, Fable 5.1 — $10 and $50. The cache has prices of its own, the columns "5m cache writes", "1h cache writes" and "Cache hits and refreshes".
- **OpenAI:**
  - [Spend limits](https://developers.openai.com/api/docs/guides/spend-limits):
    - "To stop traffic after tracked spend reaches a configured amount, enforce a hard spend limit for your organization or an individual project.";
    - "Spend alerts do not enforce a cap." In the table of the same page, the hard limit has "Affected API requests return a `429` error" and "Enforce a monthly organization or project cap". The switch "Enforce a hard limit" turns it on;
    - "Enforcement is not instantaneous, so recorded spend can slightly exceed the configured amount.";
    - "Otherwise, the limit resets with the next monthly cycle."
  - [Error codes](https://developers.openai.com/api/docs/guides/error-codes): every refusal for the account has the status 429, and the codes are `credit_balance_exhausted`, `organization_spend_limit_exceeded`, `project_spend_limit_exceeded` and `organization_usage_limit_exceeded`. More:
    - "For billing-related errors, inspect `error.code` to identify the specific cause. The broader `error.type` can still be `insufficient_quota`.";
    - "Retrying billing, spend, or quota errors won't restore API access."
  - [Rate limits](https://developers.openai.com/api/docs/guides/rate-limits): "OpenAI sets an approved monthly usage limit for each organization." By tier, from "$100 / month" at Free to "$200,000 / month" at Grow.
  - [Create a response](https://developers.openai.com/api/reference/resources/responses/methods/create), `usage`: `input_tokens_details` is "A detailed breakdown of the input tokens", and it has `cache_write_tokens` and `cached_tokens`, so the tokens of the cache are part of `input_tokens`; this is our inference from the word "breakdown". `output_tokens_details` has `reasoning_tokens`.
  - [Reasoning models](https://developers.openai.com/api/docs/guides/reasoning): "While reasoning tokens are not visible via the API, they still occupy space in the model's context window and are billed as output tokens." Anthropic has the same, the page Thinking: "the tokens Claude spends reasoning are billed as output tokens".
  - [Administration Overview](https://developers.openai.com/api/reference/administration/overview): "To access these endpoints, create an Admin API key." The usage of an organization is given by the endpoints `/organization/usage/…` of the same reference of the Administration API, with `$OPENAI_ADMIN_KEY` in the examples, and the limit by `/organization/spend_limit`: "Get the organization's hard spend limit.", also with `$OPENAI_ADMIN_KEY`. That an ordinary key does not read them the reference does not say outright: this is our inference.
  - [Pricing](https://developers.openai.com/api/docs/pricing), "Standard pricing data": `gpt-6-astra` has $10.00 input and $50.00 output on a short context, as in the adapter GPT. A write to the cache has two prices, "30m write" and "12h write", and a long context has prices of its own.
- **SDK** — the sources in the module cache, the file `internal/requestconfig/requestconfig.go` of each: anthropic-sdk-go v1.84.0 and openai-go v3.85.0 retry a request twice by default, `MaxRetries: 2`. They retry the responses 408, 409, 429 and 5xx, unless the response carries `x-should-retry: false`, and 400 and 402 only if the response carries `x-should-retry: true`. Whether a provider sends `x-should-retry: false` with a refusal for a limit, the documentation does not say.

### 1.13. Admitted models and retention at Anthropic, 2026-11-08

The sources of architecture 3.9, the rule of admission and the list of admitted models (T113). Read 2026-11-08, by the `.md` versions of the pages of the documentation; the quotes are verbatim, what is retold is ours.
- [API and data retention](https://platform.claude.com/docs/en/manage-claude/api-and-data-retention), no date on the page: "Claude Fable 5.1, Claude Mythos 5.1, Claude Fable 5, and Claude Mythos 5: These models require 30-day data retention and are not available under ZDR unless expressly authorized by Anthropic." The same page: "Conversation content (your prompts and Claude's outputs) is not retained by default; the exception is Covered Models, which require 30-day retention." The page does not name Opus 5.5, Sonnet 5.5 and Haiku 5.5 among such models.
- [Covered Models](https://support.claude.com/en/articles/15425695-covered-models), an article of 2026-09-01: "Note: These policies apply only to the models listed on this page. All other Claude models continue to operate under your existing agreement and configured retention settings."
- [How long do you store my organization's data?](https://privacy.claude.com/en/articles/7996866-how-long-do-you-store-my-organization-s-data), an article of 2026-07-01: "For Anthropic API users, we automatically delete inputs and outputs on our backend within 30 days of receipt or generation, except…" — then the article lists the exceptions, the agreement on zero data retention among them. It disagrees with the documentation above, which says that the content is not kept by default. For the list of admitted models this does not matter — minimal retention is stated by the deployment fact `minimal_retention` of the admission table — but such a fact is to be read by the agreement, not by one of the pages.
- [Models overview](https://platform.claude.com/docs/en/models/overview): "| Claude API ID | `claude-fable-5-1` | `claude-opus-5-5` | `claude-sonnet-5-5` | `claude-haiku-5-5` |".

### 1.14. Prices and cache, 2026-11-11

The sources of the table of prices of architecture 3.7 (T120). Read 2026-11-11: at Anthropic by the `.md` versions of the pages, at OpenAI by the pricing page as the server gives it. The quotes are verbatim:
- **Anthropic:**
  - [Pricing](https://platform.claude.com/docs/en/about-claude/pricing), the table "Model pricing", per 1M tokens — input, a write to the cache for 5 minutes and for an hour, a cache hit, output: Opus 5.5 — $4, $5, $8, $0.20, $20 (the same on 2026-11-13, before the trial of T143); Sonnet 5.5 — $2, $2.50, $4, $0.10, $10; Fable 5.1 — $10, $12.50, $20, $0.25, $50; Haiku 5.5 "(for prompts up to 100,000 tokens)" — $0.10, $0.125, $0.20, $0.01, $0.50, and "(for prompts over 100,000 tokens)" — $0.50, $0.625, $1, $0.05, $2.50;
  - the same page: "Claude Haiku 5.5 is priced by prompt length: a request whose prompt is over 100,000 tokens pays higher prices. A request’s prompt length counts all of its input tokens, including cache reads and cache writes." And on the rest: "Claude 4.6 and later models (except Claude Haiku 5.5) and Claude Mythos Preview include the full 1M token context window at standard pricing.";
  - [Prompt caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching): "By default, the cache has a 5-minute lifetime." How the adapter counts a write that `usage` does not divide by its lifetime is architecture 3.7;
- **OpenAI:**
  - [Pricing](https://developers.openai.com/api/docs/pricing), "Standard", a short context: `gpt-6-astra` has input $10.00, output $50.00, "Cached input" $1.00, "30m write" $12.50 and "12h write" $25.00. The page gives the prices of a long context — above "272K input tokens" — only by a script, and they could not be read; what the adapter does above this threshold is architecture 3.7;
  - [Prompt caching](https://developers.openai.com/api/docs/guides/prompt-caching), an example of a response: `"input_tokens" : 15000 , "input_tokens_details" : { "cached_tokens" : 12000 , "cache_write_tokens" : 3000 }`. So both what is read from the cache and what is written to it are part of `input_tokens` (architecture 3.7). The writes by lifetime are `cache_write_tokens_details`, `ttl_30m_tokens` and `ttl_12h_tokens` in the types of openai-go v3.94.0.

### 1.15. Refusals, 2026-11-11

The sources of the recognition of refusals of architecture 3.7 (T121). Read 2026-11-11, at Anthropic by the `.md` versions of the pages. The quotes are verbatim:
- **Anthropic:**
  - [Errors](https://platform.claude.com/docs/en/api/errors): "401 - `authentication_error`: There’s an issue with your API key (for example, it’s malformed, revoked, or expired; see Key expiration)."; "A 401 whose message begins `CORS requests are not allowed for this Organization` is not a problem with your API key."; "402 - `billing_error`: There’s an issue with your billing or payment information."; "403 - `permission_error`: Your API key does not have permission to use the specified resource.";
  - [Rate limits](https://platform.claude.com/docs/en/api/rate-limits): for the cap of a tier, "The error type is `rate_limit_error`, the same as for a rate limit, but the response has no `retry-after` header. Retrying, including the SDK’s automatic retries, fails until access resumes." and "On the Messages API, `error.details.error_code` is `enforced_spend_limit_reached`."; for the owner's limit, "requests return HTTP 400 with error type `invalid_request_error`. The message starts with `You have reached your specified API usage limits`, or `You have reached your specified workspace API usage limits` for a workspace limit";
- **OpenAI** — [Error codes](https://developers.openai.com/api/docs/guides/error-codes): "401 - Invalid Authentication", "401 - Incorrect API key provided" — "The requesting API key is not correct.", and among the causes of the first, "You are using a revoked API key."; the codes of refusals for the account: "429 - Credit balance exhausted", `credit_balance_exhausted`; "429 - Organization spend limit reached", `organization_spend_limit_exceeded`; "429 - Project spend limit reached", `project_spend_limit_exceeded`; "429 - Organization usage limit reached", `organization_usage_limit_exceeded` — "Your organization reached its OpenAI-assigned usage limit.";
- **SDK** — anthropic-sdk-go v1.84.0 and openai-go v3.94.0, `internal/requestconfig/requestconfig.go`: the header of the response `x-should-retry` decides whether to retry a request before the code of the response does — "If the header explicitly wants a retry behavior, respect that over the http status code." in the SDK of Anthropic.

### 1.16. The own key check, 2026-11-11

The sources of `Tutor.CheckKey` of architecture 3.7 (T122). Read 2026-11-11, by the `.md` versions of the pages:
- **Anthropic** — [Token counting](https://platform.claude.com/docs/en/build-with-claude/token-counting): "Token counting is **free to use** but subject to requests per minute rate limits based on your usage tier." and "Token counting and message creation have separate and independent rate limits." The code 403: "Your API key does not have permission to use the specified resource." ([Errors](https://platform.claude.com/docs/en/api/errors));
- **OpenAI** — [List models](https://developers.openai.com/api/reference/resources/models/methods/list): "Lists the currently available models, and provides basic information about each one such as the owner and availability." The pages name no price for a request to the models, and [Pricing](https://developers.openai.com/api/docs/pricing) charges for tokens: that the description of a model is free is our inference.

### 1.17. The SDKs and the process's environment, 2026-11-13

The sources of the adapters without the environment's defaults of architecture 3.2 (T137; R105): the code of the two SDKs at the versions of `go.mod`, anthropic-sdk-go v1.84.0 and openai-go v3.94.0, read 2026-11-13 in the module cache. The quotes are verbatim:
- **Anthropic, the defaults** — `client.go`: `NewClient` begins "if option.HasWithoutEnvironmentDefaults(opts) { defaults = []option.RequestOption{option.WithEnvironmentProduction()} } else { defaults = DefaultClientOptions() }". `DefaultClientOptions` reads `ANTHROPIC_BASE_URL`, `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_PROFILE`, the federation — "Env-var federation (ANTHROPIC_FEDERATION_RULE_ID + ANTHROPIC_ORGANIZATION_ID + ANTHROPIC_IDENTITY_TOKEN_FILE / _TOKEN)", with `ANTHROPIC_SERVICE_ACCOUNT_ID` and `ANTHROPIC_WORKSPACE_ID` in `internal/auth/default.go` —, a `Fallback profile (active_config file or literal "default"`, `ANTHROPIC_WEBHOOK_SIGNING_KEY` and `ANTHROPIC_CUSTOM_HEADERS`, and puts first "option.WithHTTPClient(requestconfig.DefaultHTTPClient())". A profile is read by package `config`, which reads `ANTHROPIC_CONFIG_DIR`, `APPDATA`, `XDG_CONFIG_HOME` and `ANTHROPIC_SCOPE` too. `option/requestoption.go`: "WithoutEnvironmentDefaults returns a marker [RequestOption] that, when passed to anthropic.NewClient, causes it to skip the environment-based credential autoload performed by anthropic.DefaultClientOptions (ANTHROPIC_API_KEY, ANTHROPIC_AUTH_TOKEN, ANTHROPIC_PROFILE, env federation, fallback profile, ANTHROPIC_BASE_URL). The hardcoded production base-URL default is still applied". The client of the defaults, `internal/requestconfig/defaultclient.go`: "a clone of [http.DefaultTransport] with a response-header timeout, or [http.DefaultTransport] itself if it has been replaced by a wrapper such as otelhttp", and "const defaultResponseHeaderTimeout = 10 * time.Minute"; without it a request goes by `http.DefaultClient`.
- **Anthropic, the tracing** — `option/tracing.go`: "The client emits spans without this option, through the registered provider."; "WithoutOpenTelemetry returns a RequestOption that turns off the OpenTelemetry spans of API calls. It is the short form of WithOpenTelemetryTraces(false)."; `WithOpenTelemetryTraces` "turns the OpenTelemetry spans of API calls, and the trace-context headers of their requests, on or off. They are on by default."; `Without this option, the environment variable ANTHROPIC_OPEN_TELEMETRY_TRACES_CONTENT_MODE selects [OpenTelemetryContentModeContent] when its value is "content".` `internal/tracing/tracing.go`, `Tracer`: "if o.Traces.Enabled != nil && !*o.Traces.Enabled { return nil }", before it reads `ANTHROPIC_OPEN_TELEMETRY` or the process's provider, `otel.GetTracerProvider()`; `StartAPICallSpan` returns nil without a tracer, and the request goes without a span and without the trace context. `ANTHROPIC_OPEN_TELEMETRY_PROPAGATION` and `ANTHROPIC_OPEN_TELEMETRY_TRACES_MAX_CONTENT_BYTES` are read only by a span.
- **OpenAI** — `client.go`: "DefaultClientOptions read from the environment (OPENAI_API_KEY, OPENAI_ADMIN_KEY, OPENAI_ORG_ID, OPENAI_PROJECT_ID, OPENAI_WEBHOOK_SECRET, OPENAI_BASE_URL)", and `OPENAI_CUSTOM_HEADERS` besides; `NewClient` begins "defaults := DefaultClientOptions() if requestconfig.EnvironmentDefaultsDisabled(opts...) { defaults = requestconfig.InheritedOptions(defaultClientOptionsWithoutEnvironment()...) }", where "return []option.RequestOption{option.WithHTTPClient(defaultHTTPClient()), option.WithEnvironmentProduction()}", the same client with the same 10 minutes (`default_http_client.go`). The marker is internal: `internal/requestconfig/requestconfig.go`, "func WithEnvironmentDefaultsDisabled(opts ...RequestOption) RequestOption". The services read no variable: "func NewResponseService(opts ...option.RequestOption) (r ResponseService)" and `NewModelService` keep the options, and `ResponseService.New` and `ModelService.Get` put first "requestconfig.WithBearerAuthSecurity()". openai-go has no OpenTelemetry.
- **What else reads the environment** — in packages the adapters do not import: `bedrock`, `vertex`, `foundry`, `googlecloud` and `internal/awsauth` of Anthropic (`ANTHROPIC_AWS_…`, `ANTHROPIC_BEDROCK_MANTLE_BASE_URL`, `ANTHROPIC_FOUNDRY_…`, `ANTHROPIC_GOOGLE_CLOUD_…`, `AWS_REGION`, `AWS_DEFAULT_REGION`, `AWS_BEARER_TOKEN_BEDROCK`, `AWS_BEDROCK_BASE_URL`, `GOOGLE_CLOUD_PROJECT`), `lib/environments` (`ANTHROPIC_WORK_…`, `ANTHROPIC_ENVIRONMENT_…`, `ANTHROPIC_SESSION_ID`) and `tools/agenttoolset`, and `bedrock` of OpenAI; the SDKs' tests (`ANTHROPIC_LIVE`, `UPDATE_GOLDEN`, `SKIP_MOCK_TESTS`). Go's transport, not the SDKs, reads `HTTPS_PROXY`, `HTTP_PROXY` and `NO_PROXY` (`http.ProxyFromEnvironment` of `http.DefaultTransport`) and `SSL_CERT_FILE` and `SSL_CERT_DIR` (`crypto/x509`).

## 2. Reading handwriting elsewhere

### 2.1. The market: how others read a handwritten solution, 2026-10-25

The sources of architecture 3.12 (T41). Read 2026-10-25.
- The quotes were taken from the text of the pages themselves by `curl`. The help pages of Gradescope, Khan Academy and Goodnotes answered a request for the page with 403, and their text was taken from the public JSON of the same articles (the API of Zendesk).
- In brackets is the date of the page itself, if it has one.
- Everything below is what the companies themselves state, except what is named outright as a retelling of the press, a secondary source or someone else's review.

- **MyScript** (iink SDK, MyScript Math):
  - [prices](https://developer.myscript.com/pricing/): "On-device recognition Includes all the features of MyScript iink SDK: our SDK for native applications. Pricing is based on licenses per device."; in the cloud, "Free 2,000 requests, for testing and development only", and beyond that by agreement;
  - [the cloud](https://developer.myscript.com/docs/cloud/4.5/overview/introduction/): "MyScript technology processes all recognition operations on the server side."; "MyScript Cloud servers are deployed on AWS in Oregon";
  - [the blog of iink SDK 3.0](https://www.myscript.com/blog/iink-sdk-3-0-math/) (2024-04-26): "The new architecture comprises an end-to-end model, functioning as an encoder-decoder transformer based on the multi-head attention mechanism.";
  - [the data policy](https://www.myscript.com/privacy-policy/) ("Version: May 2026"), on MyScript Math: "The AI features are powered by OpenAI.";
  - [the help of MyScript Math](https://help.myscript.com/math/create-and-manage-your-content/write-and-solve-calculations/): "Leave your calculation unfinished with an '=' sign, and the app automatically solves it for you, as long as all values in the calculation are defined."; "Yes, step-by-step solutions require both an internet connection and a signed-in MyScript account." The time of a response is not published.
- **Mathpix**:
  - [OCR](https://mathpix.com/ocr): "<0.5s Median image response"; "200ms Digital ink, recognized as you draw"; "Latency figures measured on live customer traffic, not a curated test set."; "There is an on-premise deployment for documents that cannot leave your network";
  - [the API's prices](https://mathpix.com/pricing/api): by strokes, "$0.002 / request", and in the table of tiers, "1M+ requests $0.0015"; "Volume tiers reset each calendar month"; by picture, "$0.002 / image", and above a million, "1M+ images $0.0015"; "Digital ink billed against a stroke session ID is priced per session instead: free up to 1K, then $0.01 (1K–100K), $0.008 (100K–1M), and $0.005 above 1M.";
  - [data](https://mathpix.com/privacy) ("Last Updated: August 5, 2024"): "By default, we collect information like requests and responses for QA purposes."; "You can opt-out of any images being saved to disk for QA purposes from your API key by setting the improve_mathpix flag in your API requests to false". The flag speaks of pictures;
  - [strokes](https://docs.mathpix.com/guides/strokes): "Recognition quality depends heavily on how many points your client actually captures."
- **Google ML Kit Digital Ink Recognition**:
  - [the overview](https://developers.google.com/ml-kit/vision/digital-ink-recognition) (2024-07-10): "The user's handwriting stays on the device, recognition is performed without any network connection";
  - [models](https://developers.google.com/ml-kit/vision/digital-ink-recognition/base-models) (2024-07-10): "models that can recognize text in over 300 languages, as well as models that classify symbols and shapes". The list has no model for mathematics;
  - [Android](https://developers.google.com/ml-kit/vision/digital-ink-recognition/android) (2026-10-23): "Around 20MB of storage is required per language.";
  - [the blog](https://developers.googleblog.com/digital-ink-recognition-in-ml-kit/) (2020-08-06): "On a typical mobile device recognizing a line of text takes about 100 ms."
- **Apple Math Notes** (iPadOS 18 and later):
  - [the help](https://support.apple.com/guide/ipad/solve-math-with-math-notes-ipadeb38d0f8/ipados): "Tap the equal sign to see what Math Notes understood in what you wrote."; "Tap any dotted blue lines to clarify what you wrote";
  - [Newsroom](https://www.apple.com/newsroom/2024/06/ipados-18-introduces-powerful-intelligence-features-and-apps-for-apple-pencil/) (2024-06-10): "see them instantly solved in their own handwriting";
  - a retelling of the press, [9to5Mac](https://9to5mac.com/2024/07/17/ipados-18-smart-script-math-notes-interview/) (2024-07-17), the words of a product manager of Apple: "It combines the natural input of pencil with the amazing on-device machine learning models". Apple's primary sources on Math Notes do not say this;
  - the recognizer of PencilKit, which Apple opened in iOS 27, is for text only ([WWDC26](https://developer.apple.com/videos/play/wwdc2026/203/)): "Handwriting recognition in PencilKit runs entirely on device." There is no API for mathematics.
- **Microsoft OneNote Math Assistant**:
  - [connected experiences](https://learn.microsoft.com/en-us/microsoft-365-apps/privacy/connected-experiences-content) (2026-05-18): the row "Math Assistant" in the table of what analyses the content; "a connected experience sends content to Microsoft so that the connected experience can perform its function";
  - [the help](https://support.microsoft.com/en-us/onenote/onenote-help-and-learning/solve-math-equations-with-math-assistant-in-onenote): "Math Assistant in OneNote can solve it for you—helping you reach the solution quickly, or displaying step-by-step instructions"; "To learn how OneNote solved the problem, you can click or tap Show steps";
  - [the help on ink](https://support.microsoft.com/en-us/education/onenote/create-math-equations-using-ink-or-text-with-math-assistant-in-onenote): "Sometimes the Math tool misunderstands handwriting.";
  - [Azure Ink Recognizer](https://learn.microsoft.com/en-us/previous-versions/azure/cognitive-services/Ink-Recognizer/): "fully retired on January 31, 2021". There is no public API, and the time of a response is not published.
- **Samsung Notes** (Galaxy AI, Solve Math):
  - [Samsung Newsroom](https://news.samsung.com/global/user-guide-galaxy-tab-s10-fe-the-ultimate-study-buddy-for-exam-season) (2025-04-03): "By writing a formula followed by an equal sign (=), the answer appears automatically."; a footnote: "Solve Math runs on-device and does not require an internet connection but requires a Samsung Account login.";
  - the page [Note Assist](https://www.samsung.com/us/support/answer/ANS10000941/): "Write equations ending with an equal sign, and Galaxy AI will provide instant solutions."; its general footnote is "A Samsung account and internet connection is required." This footnote disagrees with the footnote of the Newsroom;
  - the steps of a solution come through Circle to Search with Google ([the help](https://www.samsung.com/us/support/answer/ANS10007364/)): "Galaxy AI will solve the equation and provide step-by-step instructions."
- **小猿口算 (Xiaoyuan Kousuan)**, a check of homework sums by a photo:
  - [App Store](https://apps.apple.com/cn/app/id1325419855), version 3.144.2 (2026-10-19), under a new name: "小猿AI - 原「小猿口算」检查作业神器"; "作业检查/批改：拍照秒判对错，支持全学科、全学段100+题型批改，自动标注错误点，支持错因分析与错题讲解。" — by a photo, "correct" or "incorrect" at once, with the error marked and explained; "融合自研猿力大模型与deepseek大模型，精准诊断学生薄弱点" — its own large model and DeepSeek are taken to diagnose the weak points;
  - [the site](https://xiaoyuankousuan.com/): "拍照1秒批改全科作业" — homework checked in a second;
  - [雷锋网](https://www.leiphone.com/category/smarteducation/SMg6uhbPpdS5ez26.html) (2019-11-19), a retelling of the company's words: "系统接收用户上传的图片后" — the system receives the uploaded picture; "针对不同题型，系统采用专业的深度学习识别算法与批改策略" — each type of task has its own recognition and rules of checking; "技术上主要采用的是通用OCR技术。" — at the base is ordinary text recognition; "不到一秒钟，整页的检查结果就会反馈在屏幕上" — a page is checked in less than a second;
  - someone else's review without a method, [aixq.cc](https://www.aixq.cc/27261.html) (2026-05-08): "第一次使用时拍一张数学卷子，AI大概3到5秒就出结果" — by a photo of a maths test, the result in about 3–5 seconds.
- **作业帮 (Zuoyebang)**, a check of homework by a photo:
  - [App Store](https://apps.apple.com/cn/app/id803781859), version 14.54.0 (2026-10-10): "AI秒批作业：一键秒批全科作业，涵盖作业批改、口算批改和作文批改，错题解析+原题精讲" — an instant check, then an analysis of the errors;
  - [news.qq.com](https://news.qq.com/rain/a/20201224A0E0UD00) (2020-12-24), a talk of the head of architecture as retold by PConline: "在作业帮的整套产品链条中，拍搜系统提供了底层的技术支持" — the whole line of products stands on the system of photo search; "从搜索到批改再到打分" — from the search of tasks to checking and grading; "作业帮拍搜系统拥有超30种不同的神经网络各司其职" — more than 30 neural networks; "大量使用GPU作为运算设备的深度学习在线推理服务" — recognition runs on servers with GPUs; "平均耗时从3秒以上缩短至300多毫秒，后续持续优化到平均200毫秒" — recognition went down from 3 s to 200 ms on average;
  - [Sohu](https://www.sohu.com/a/366829682_120154852) (2020-01-14), a retelling of the press: "计算类题型全批改，批改整体准确率达 98%" — tasks of calculation are checked in full, with an accuracy of 98%;
  - [duozhi.com](http://www.duozhi.com/industry/insight/2024120616804.shtml) (2024-12-06), the press: "基于银河大模型，S30支持小初高全科作业批改" — on the company's learning tablet S30, work is checked on the basis of its large model. Where the model runs is not said.
- **Photomath** (Google):
  - [the help](https://support.google.com/photomath/answer/14328660?hl=en): "Through the years Photomath switched focus from on-device to cloud computing. Each image with a math problem is now sent to cloud servers and is being analyzed by a highly advanced neural network.";
  - the same page: "a problem-solving algorithm is applied to the formula to provide the solution and solving steps". A program solves the task; the help says nothing of checking the learner's work;
  - the same page: "recognize math problems ranging from arithmetic to calculus instantly"; in the paid Plus, "AI-powered animated Tutorials to visualize each step".
- **Magma Math**, mathematics at school, the solution on a canvas:
  - [the product](https://www.magmamath.com/product): "Magma automatically grades both typed and handwritten answers.";
  - [the list of operators](https://www.magmamath.com/ca/legal-documents/list-of-operators) of the Canadian legal entity ("Last updated: June 16, 2026"): "Google LLC (Google Gemini) | AI inference for instructional feedback and handwriting recognition | De-identified prompts only; no directly identifying student information | Outside Canada" — four cells of one row of a table, separated here by " | ". By this row Gemini reads the handwriting, and that it does so on a server is our inference;
  - [Lava](https://intercom.help/magma-math-us/en/articles/11427417-how-lava-supports-student-learning) (2026-09-25): "Lava acts as a first layer of feedback during assignments."; "When a student submits an incorrect response, Lava can provide" — then a list of hints; "If a student answers correctly but the drawing board only contains scribbles, dots, or other work that does not demonstrate their thinking, the Submit button changes to Verify." The time of a response is not published.
- **Gradescope** (Turnitin):
  - [AI-assisted grading](https://guides.gradescope.com/hc/en-us/articles/24838908062093-AI-assisted-grading-and-answer-groups) (2025-12-16): "Gradescope AI automatically forms suggested answer groups for your review."; "Gradescope AI can read student handwriting of English-language text and math notation … The primary constraint is for the student's answer to be on one line". The teacher gives the grade;
  - a secondary source, [Stanford Teaching Commons](https://teachingcommons.stanford.edu/opportunities/gradescope): "These features do not use generative AI technology." The time of processing is not published;
  - [Public API](https://guides.gradescope.com/hc/en-us/articles/36028522325901-Gradescope-Public-API) (2025-04-22): "Unfortunately, we don’t have a public API".
- **Khanmigo** (Khan Academy):
  - [the help](https://support.khanacademy.org/hc/en-us/articles/36868912022541) (2025-10-06): "A photo of your handwritten math problem to check your steps";
  - [a paper of Khan Academy](https://arxiv.org/pdf/2608.11259), the full text in PDF (AIED 2026, submitted 2026-08-07):
    - "This workflow uses a Python Math Agent to run and check student math on each message to reduce potential LLM math mistakes.";
    - "a Math Agent which can run student input math in a sandbox" — with NumPy and SymPy;
    - "Tutor Me uses a reasoning LLM (o3-mini; OpenAI models, n.d.) to generate a solution and step-by-step hints for the AI tutor to reference.";
    - on moving the input classifier to GPT-4.1-mini: "Time to first token latency (P50) decreased by 14.40% ± 1.30%". In the table of results, the time to the first token and the time to the full response are separate rows; the second is "Time to full response Latency (P50)";
  - [the blog](https://blog.khanacademy.org/how-khan-academy-is-building-a-better-ai-tutor-our-most-recent-learnings/) (2026-05-01): "Instructing the math agent to produce a more concise response to Khanmigo reduced mean response time by three seconds across 352,000 tutoring threads over five days."; "A follow-up experiment in which we limited the agent to focusing on the math the student had already done instead of also working out the remaining steps to get to the solution reduced latency by another 400 milliseconds and reduced giving away the answer by 50%."; "Switching the math agent to a faster AI model reduced response time by 0.3 seconds across 1.35 million tutoring threads over 12 days."; a pre-check of whether the Math Agent is needed, "cutting response time by about 0.3 seconds"; the levers, "a faster model, shorter outputs, tighter time-outs, and smarter routing".
- **Snorkl**, the learner explains the solution on a board by voice:
  - [the profile of iKeepSafe](https://ikeepsafe.org/content/uploads/2024/05/Snorkl-Product-Profile.pdf) (2024-05-16): "We use OpenAI's API to power the grading and feedback features in Snorkl.";
  - [scores](https://help.snorkl.app/en/articles/12675870-understanding-snorkl-ai-scores) (2025-11-02): "Correctness – whether the student’s final answer, claim, or conclusion is accurate. Explanation Score (0–4) – how complete, clear, and thorough their reasoning or explanation is.";
  - [the site](https://snorkl.app/): "providing instant AI feedback on recorded explanations". The seconds are not published.
- **Goodnotes**, a handwritten notebook:
  - [the blog](https://www.goodnotes.com/blog/how-its-made-ai-math-assistance) (2023-11-17): "AI Math Assistance runs locally on your device so that all data stays private to you."; "checking your handwritten math work in real-time";
  - [credits](https://support.goodnotes.com/hc/en-us/articles/13817639294223) (2025-12-03): "Several helpful AI features run right on your device without using any credits: Math Conversion, Circle to Lasso, Spell Check, Handwriting Reflow, and Scribble to Erase."; "The more advanced cloud-powered features (such as Goodnotes AI for Whiteboard/Notebook, Text Document, Math, and Meetings) will count toward your monthly credit usage.";
  - [data](https://support.goodnotes.com/hc/en-us/articles/9795720752911) (2025-09-19): "Goodnotes AI uses an external large language model hosted on Amazon Bedrock";
  - [the blog](https://www.goodnotes.com/blog/math-ai-that-builds-understanding) (2025-09-29): "Goodnotes AI handles the conversation and explanations with one model, then draws on Wolfram|Alpha’s capabilities for thinking through the math."; "Before any computation happens, Goodnotes AI shows you exactly how it interpreted your problem with a confirmation card.";
  - [the assistant](https://support.goodnotes.com/hc/en-us/articles/10779112528399) (2026-03-17), on requests to Goodnotes AI in general, not only on mathematics: "More complex requests involving multiple steps may take a few minutes to complete."
- **MathGPT.ai**, a check of university work, [a press release](https://www.prnewswire.com/news-releases/mathgptai-automates-grading-for-handwritten-higher-education-math-assignments-302838255.html) (2026-07-29): "It processes unstructured, handwritten student math solutions and proofs in minutes, automatically analyzing uploaded paper-based work to factor it towards student grades, even when the final answer submitted is incorrect."
- **Brisk**, a teacher's tool, [the page](https://www.briskteaching.com/post/upload-photos-to-create-lesson-content-and-generate-feedback): "Upload a photo of a student’s handwritten work, and Brisk will analyze it, offering suggestions, corrections, or even full feedback in seconds". The models, and where it reads the work, are not published.
- **Set aside** — they solve the task from a photo and do not read the learner's working:
  - Thetawise: "ThetaWise reads the math and gets straight to work." ([the page](https://thetawise.ai/features/math-solver));
  - Mathos AI: "Our AI math solver and calculator recognize handwriting and complex formulas instantly." ([the site](https://www.mathos.ai/)).

### 2.2. Third-party measurements of speed, 2026-10-25

The sources of architecture 3.12 (T42, T44). Read 2026-10-25. These are measurements on other people's tasks, not a check of the board.
- **Artificial Analysis:**
  - [the method](https://artificialanalysis.ai/methodology/performance-benchmarking): "Vision workload | A single 1 megapixel image and approximately 1,000 input tokens, 1,000 output tokens" — the cells of the table's row separated here by " | "; "median (P50) over the past 72 hours";
  - on speed: "we calculate output speed using the last 80% of answer chunks"; the tokens are counted by the "tiktoken library (o200k_base)", and the server stands in "Google Cloud's us-central1-a";
  - on the pages of the models, on the time to the first token of the answer: "For reasoning models, this includes the 'thinking' time of the model before providing an answer." At Claude the data of the pages do not single out the time of thinking, `reasoningTime` is 0. Our inference: it is part of the time to the first token;
  - the numbers of the workload with a picture are from the data embedded in the pages of the models, by the row of the API of Anthropic. For example, at [Opus 5.5 · high](https://artificialanalysis.ai/models/claude-opus-5-5-high/providers): `"visionSingleImage":{"medianOutputSpeed":56.691…,"medianTimeToFirstChunk":7.8973…`. So too the pages [Haiku 5.5 · low](https://artificialanalysis.ai/models/claude-haiku-5-5-low/providers), [Sonnet 5.5 · low](https://artificialanalysis.ai/models/claude-sonnet-5-5-low/providers), [Opus 5.5 · low](https://artificialanalysis.ai/models/claude-opus-5-5-low/providers) and [Fable 5.1 · low](https://artificialanalysis.ai/models/claude-fable-5-1-low/providers). In architecture 3.12 the numbers are rounded. The variants of Sonnet, Opus and Fable are named "with fallback" there.
- **OpenRouter**, the pages of the models — medians of live traffic, a slice of 2026-10-25 at about 05:19 UTC. The numbers change every few minutes:
  - [Opus 5.5](https://openrouter.ai/anthropic/claude-opus-5.5): the row Anthropic has the cells "$4.00", "$20.00", "$0.20", "3.02s" and "80 tps", and the row Anthropic marked "Fast" has "$8.00", "$40.00", "$0.40", "1.72s" and "133 tps";
  - so too [Haiku 5.5](https://openrouter.ai/anthropic/claude-haiku-5.5), [Sonnet 5.5](https://openrouter.ai/anthropic/claude-sonnet-5.5) and [Fable 5.1](https://openrouter.ai/anthropic/claude-fable-5.1). The pages' filter "Reasoning effort" shows all the levels at once;
  - the hints of the columns, from the page's script: "Median time to first token, measured from when the request is sent. Data from the last 30 minutes."; on speed, "measured from when the request is sent to the end of the response, so time to first token is included. Includes reasoning tokens. Only responses with more than 50 output tokens count."
- **Artificial Analysis, for T44** — the same workload with a picture, from the data of the pages, by the row of the provider itself:
  - [GPT-6 Luna, without reasoning](https://artificialanalysis.ai/models/gpt-6-luna-non-reasoning/providers): `"visionSingleImage":{"medianOutputSpeed":96.6567964259013,"medianTimeToFirstChunk":1.25676392999995`. Our inference: this is effort `none` — Luna has no other setting without reasoning;
  - so too [GPT-6 Luna · low](https://artificialanalysis.ai/models/gpt-6-luna-low/providers): 1.92 s and 103.8 tokens a second; [GPT-6 Luna · medium](https://artificialanalysis.ai/models/gpt-6-luna-medium/providers): 3.01 s and 68.1; [GPT-6 Astra · low](https://artificialanalysis.ai/models/gpt-6-astra-low/providers): 1.97 s and 36.5;
  - [Gemma 4 31B, without reasoning](https://artificialanalysis.ai/models/gemma-4-31b-non-reasoning/providers), the row SambaNova: `"medianOutputSpeed":393.250138980534,"medianTimeToFirstChunk":2.40469330750005`;
  - for Groq, Cerebras, Together and Fireworks, Artificial Analysis has no numbers on the models with pictures. OpenRouter does not show them without a key: "Only visible when authenticated with an API key or cookie; returns null for unauthenticated requests." ([the list of endpoints](https://openrouter.ai/docs/api/api-reference/endpoints/list-all-endpoints-for-a-model)).
- **Read again 2026-11-13** (T140; Д12): the numbers of the table "where the seconds go" of architecture 3.12 that the items above do not give raw.
  - **Artificial Analysis no longer measures the workload with a picture.** [The method](https://artificialanalysis.ai/methodology/performance-benchmarking), "Version 3.0.0 3 November 2026": "These replace the vision workload and the single and parallel prompt load scenarios." The pages of the models of 2026-11-13 have only the workloads "1k input tokens", "10k input tokens", "100k input tokens" and "Multi-turn", so the numbers of 2026-10-25 are no longer at the source.
  - **Its copies in the Wayback Machine** — the pages of the models, the row Anthropic, `"visionSingleImage":{"medianOutputSpeed":…,"medianTimeToFirstChunk":…`, the copy nearest to 2026-10-25 of each:
    - [Haiku 5.5 · low, 2026-10-09 08:30 UTC](http://web.archive.org/web/20261009083023/https://artificialanalysis.ai/models/claude-haiku-5-5-low/providers): 113.968031591816 and 3.63281576200001; architecture 3.12 has 116 and 4.05 s;
    - [Sonnet 5.5 · low, 2026-09-29 18:38 UTC](http://web.archive.org/web/20260929183848/https://artificialanalysis.ai/models/claude-sonnet-5-5-low/providers): 73.1696702895911 and 0.963358530999983; architecture 3.12 has 73 and 1.17 s;
    - [Opus 5.5 · low, 2026-10-25 18:52 UTC](http://web.archive.org/web/20261025185220/https://artificialanalysis.ai/models/claude-opus-5-5-low/providers): 55.452985737401 and 2.66235274000002; architecture 3.12 has 55 and 2.84 s;
    - [Opus 5.5 · high, 2026-10-25 18:53 UTC](http://web.archive.org/web/20261025185339/https://artificialanalysis.ai/models/claude-opus-5-5-high/providers): 57.0530539740461 and 7.81457842099999; the reading of T42 above has 56.691… and 7.8973…;
    - [Fable 5.1 · low, 2026-10-24 02:57 UTC](http://web.archive.org/web/20261024025738/https://artificialanalysis.ai/models/claude-fable-5-1-low/providers): 35.6583194133928 and 0.867283970499955 — the 36 and 0.87 s of architecture 3.12 after rounding.

    The medians are of 72 hours, so a copy of another hour gives other numbers. Of the five rows Opus · high is held by the reading above, and Fable by its copy of the day before, after rounding; those of Haiku, Sonnet and Opus · low no copy holds (architecture 2.6). The copies of Opus · low and · high, a minute apart, show what architecture 3.12 infers: the effort moves the time to the first token, 2.66 s against 7.81 s, and not the speed, 55.5 and 57.1.
  - **OpenRouter**, the same pages, 2026-11-13 at about 18:15 UTC, from the data the page embeds — the cells are drawn by its script and are not in its HTML —, the row Anthropic, `"p50_latency"` in milliseconds and `"p50_throughput"`: Haiku 5.5 — 1594 and 126; Sonnet 5.5 — 1193 and 111; Opus 5.5 — 2487 and 85, and its row with `"price":"0.000008"` and `"price":"0.00004"` per token, Fast mode, — 1751 and 172; Fable 5.1 — 3518 and 66. The hints of the columns stand in the page's script as quoted above, and the speed's goes on: "Data from the last 30 minutes, or the last 24 hours for providers with fewer than 50 such responses."
  - **The slice of 2026-10-25 at about 05:19 UTC** is in no copy. The nearest copies of the Wayback Machine, the row Anthropic, by the same fields: [Haiku 5.5, 2026-10-23 18:15 UTC](http://web.archive.org/web/20261023181524/https://openrouter.ai/anthropic/claude-haiku-5.5) — 1434 and 154; [Opus 5.5, 2026-10-24 00:50 UTC](http://web.archive.org/web/20261024005021/https://openrouter.ai/anthropic/claude-opus-5.5) — 3458 and 77, Fast mode 1161 and 221; [Fable 5.1, 2026-10-26 04:22 UTC](http://web.archive.org/web/20261026042245/https://openrouter.ai/anthropic/claude-fable-5.1) — 3590 and 58; [Sonnet 5.5, 2026-10-26 21:10 UTC](http://web.archive.org/web/20261026211019/https://openrouter.ai/anthropic/claude-sonnet-5.5) — 1876.5 and 100. The values of Opus 5.5 of that slice are quoted above; those of Haiku, Sonnet and Fable no copy holds (architecture 2.6).

### 2.3. On-device recognizers, 2026-10-25

The sources of architecture 3.12 (T44). Read 2026-10-25.
- **The Prompt API in Chrome:**
  - [the page of the API](https://developer.chrome.com/docs/ai/prompt-api), "Last updated: August 26, 2026": for the web, from Chrome 148; "For expectedInputs, this can be text, image, or audio."; the languages are `"en", "ja", "es", "de", and "fr"`;
  - [getting started](https://developer.chrome.com/docs/ai/get-started): "GPU: Strictly more than 4 GB of VRAM."; "CPU: 16 GB of RAM or more and 4 CPU cores or more."; "Chrome for Android, iOS, and ChromeOS on non-Chromebook Plus devices are not yet supported by the APIs which use foundation models.";
  - the size of the model is "4GB", by [Decrypt](https://decrypt.co/367193/chrome-removes-privacy-claim-gemini-nano-google), 2026-05-07: "Chrome is quietly installing a 4GB AI on your devices.";
  - the pages of Chrome on AI that were read have no conditions of age. The general terms of Google, which the footer links to, were not read. For the same model on Android, the [terms of ML Kit GenAI](https://developers.google.com/ml-kit/genai-terms) say: "You must be 18 years of age or older to use the APIs."
- **The Handwriting Recognition API** — [the article of Chrome](https://developer.chrome.com/docs/web-platform/handwriting-recognition): "It only supports English (en), and a gesture model (zxx-x-gesture) to recognize gestures such as crossing out words." In [Chrome Status](https://chromestatus.com/feature/5263213807534080): "At the moment, the recognizer is only implemented / available on certain Chrome OS devices."
- **TexTeller:**
  - [README](https://github.com/OleehyO/TexTeller): "Support scanned image, handwritten formulas, English(Chinese) mixed formulas." The code and the weights are under Apache-2.0;
  - [the card of the dataset](https://huggingface.co/datasets/OleehyO/latex-formulas-80M): "The handwritten subset of this dataset was collected entirely from existing open source work, which includes all test sets.";
  - the size is about 202 MB in the build q4f16 [for transformers.js](https://huggingface.co/onnx-community/TexTeller-ONNX). This is the sum of the sizes of the files, the executor's estimate.
- **Pix2Text-MFR 1.5** — [the card](https://huggingface.co/breezedeus/pix2text-mfr-1.5): "This MFR model utilizes the TrOCR architecture developed by Microsoft, starting with its initial values and retrained using a dataset of mathematical formula images." The license is MIT, and the data are not named. The [handwritten TrOCR](https://huggingface.co/microsoft/trocr-small-handwritten) has "TrOCR model fine-tuned on the IAM dataset".
- **SmolVLM** — [the card](https://huggingface.co/HuggingFaceTB/SmolVLM-256M-Instruct), among the prohibited uses: "Evaluating or scoring individuals (e.g., in employment, education, credit)".
- **Datasets:**
  - [MathWriting](https://github.com/google-research/google-research/tree/master/mathwriting) — "Creative Commons Attribution-NonCommercial-ShareAlike 4.0 International license";
  - CROHME — "Creative Commons Attribution-NonCommercial-ShareAlike 3.0 Unported License", by an archived copy of the page of TC11 of 2026-01-23: the page itself did not open;
  - IAM — "This database may be used for non-commercial research purpose only.", also by an archived copy.

  The ASF puts non-commercial licenses in the category X: "Non-commercial licenses: Creative Commons Non-Commercial variants" ([ASF, Third-party license policy](https://www.apache.org/legal/resolved.html)).
- **transformers.js** — [README](https://github.com/huggingface/transformers.js): "By default, when running in the browser, the model will be run on your CPU (via WASM)." `generate()` returns no probabilities of tokens: in the code at that place stands `// TODO:`.
- **Read 2026-11-13** (T140; Д12): the facts of the device that architecture 3.12 gave without a quote.
  - **WebGPU in Firefox for Android.** MDN's [browser compatibility data](https://github.com/mdn/browser-compat-data/blob/main/api/GPU.json), `api/GPU.json`, last changed 2026-07-10: in `"firefox_android"` stands `"version_added": false`. The [implementation status](https://github.com/gpuweb/gpuweb/wiki/Implementation-Status) of the WebGPU group, the section "Firefox", the column "Android" of its table: "Behind a flag", with the footnote "Disabled by default. Use Firefox Beta or Nightly and set the `about:config` setting `gfx.webgpu.ignore-blocklist` to enable. Mozilla expects to do work on Android in 2026."
  - **PP-FormulaNet** — [Formula Recognition Module Tutorial](https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/module_usage/formula_recognition.en.md) of PaddleOCR, last changed 2026-05-29. Of PP-FormulaNet_plus: "utilizes a more diverse formula dataset during training, including sources such as Chinese dissertations, professional books, textbooks, exam papers, and mathematics journals". Handwriting is not named for PP-FormulaNet, and in the same table it is for another model, UniMERNet: "including simple formulas, complex formulas, scanned formulas, and handwritten formulas".
  - **pix2tex** — [README](https://github.com/lukas-blecher/LaTeX-OCR), last changed 2024-02-12: "takes an image of a math formula and returns corresponding LaTeX code"; the data: "Luckily there is a lot of LaTeX code on the internet, e.g. wikipedia, arXiv." — the links left out; in its list of plans: "[ ] support handwritten formulae (kinda done, see training colab notebook)".
  - **Florence-2** — [the card](https://huggingface.co/microsoft/Florence-2-large): MIT, the tasks `<OCR>` and `<OCR_WITH_REGION>`, and neither formulas nor handwriting are named. The card of SmolVLM above, read again the same day, names neither either, so that these two read printed formulas their cards do not say (architecture 2.6).

## 3. Laws

### 3.1. Handwriting and pen dynamics, 2026-10-21

The primary sources of the study's scale of classes (sources 5.1) that R11 rests on. Checked 2026-10-21. The conclusion from them is our reading, not a lawyer's: the question for a lawyer is the study's `И9`. П-21 carried it, and В8 of table 3 of [RUN.md](../RUN.md) went into П-21. П-21 is closed by the decision R52 spec, and a lawyer's answer will be needed if the measurement shows that the writing process helps (R52 spec).
- [16 CFR § 312.2](https://www.ecfr.gov/current/title-16/chapter-I/subchapter-C/part-312/section-312.2), COPPA, the definition of personal information:
  - "(10) A biometric identifier that can be used for the automated or semi-automated recognition of an individual, such as fingerprints; … voiceprints; gait patterns; facial templates; or faceprints". The list is open: "such as";
  - "(11) Information concerning the child or the parents of that child that the operator collects online from the child and combines with an identifier described in this definition".
- [GDPR](http://publications.europa.eu/resource/celex/32016R0679):
  - Art. 4(14): "‘biometric data’ means personal data resulting from specific technical processing relating to the physical, physiological or behavioural characteristics of a natural person, which allow or confirm the unique identification of that natural person…";
  - recital 51: "The processing of photographs should not systematically be considered to be processing of special categories of personal data as they are covered by the definition of biometric data only when processed through a specific technical means allowing the unique identification or authentication of a natural person".
- [The Court of Justice of the EU, case C-434/16 (Nowak)](http://publications.europa.eu/resource/celex/62016CJ0434), para. 37: "In the case of a handwritten script, the answers contain, in addition, information as to his handwriting". A picture of the work is personal data even with no name on it.
- [WP29, Opinion 3/2012 (WP193)](https://ec.europa.eu/justice/article-29/documentation/opinion-recommendation/files/2012/wp193_en.pdf), para. 4.4.6, on the signature: "Typical dynamic characteristics measured by a signature biometric system (such as a digitizing tablet) are the amount of pressure, the angle of writing, velocity and acceleration of the pen…". The study carries this over to the dynamics of the pen in general.
- [ICO, Biometric recognition](https://ico.org.uk/for-organisations/uk-gdpr-guidance-and-resources/lawful-basis/biometric-data-guidance-biometric-recognition/biometric-recognition/):
  - "Behavioural characteristics can include a person’s handwriting, how they type, their gait when walking or running and their eye movements";
  - on the test "allow or confirm": "This is about the properties of the information itself, not what you intend to use it for".

## 4. The web platform

### 4.1. The browser: the pen, the time of events and the clock, 2026-10-26

The sources of the page of record mode (architecture 3.3; R25; T13). Read 2026-10-26.
- [Pointer Events Level 3](https://www.w3.org/TR/pointerevents3/), W3C Recommendation 30 June 2026:
  - pressure: "For hardware and platforms that do not support pressure, the value MUST be 0.5 when in the active buttons state and 0 otherwise";
  - release: "All pointerup events have a pressure value of 0";
  - the back end of the pen: in the tables of `button` and `buttons`, "Pen eraser button" with the values 5 and 32;
  - the primary pointer: "isPrimary Indicates if the pointer represents the primary pointer of this pointer type";
  - coalesced events: by the points of `getCoalescedEvents()` the line comes out "a smoother approximation of the pointer movement", and the events in this list have "Monotonically increasing timeStamp values". The same text: "all coalesced events have a timeStamp that is smaller than or equal to the timeStamp of the dispatched pointer event".
- [DOM Standard](https://dom.spec.whatwg.org/), "Last Updated 19 October 2026": "Returns the event’s timestamp as the number of milliseconds measured relative to the occurrence." When an event is created, its "timeStamp attribute" gets "the relative high resolution coarse time given time and event’s relevant global object".
- [High Resolution Time](https://www.w3.org/TR/hr-time-3/), W3C Working Draft 1 September 2026:
  - "The relative high resolution coarse time given a moment from the monotonic clock coarseTime and a global object global, is the duration from global’s relevant settings object’s time origin to coarseTime";
  - "The now() method MUST return the number of milliseconds in the current high resolution time given this’s relevant global object";
  - "The current high resolution time given a global object current global must return the result of relative high resolution time given unsafe shared current time and current global".

  So the time of an event and `performance.now()` count from one start, the time the page opened. That is why one clock of the sheet takes both.

## 5. MathTrail, the study and texts outside the repository

### 5.1. The study «Рукопись ребёнка на канве и ИИ-наставник», 2026-10-21

The study «Рукопись ребёнка на канве и ИИ-наставник» ("A child's handwriting on the canvas and the AI tutor") — an [artifact](https://claude.ai/artifact/QLH6WzmmQs4MY5NvjMxb1D) of 2026-10-20: the report `files/child-handwriting.md` and nine working notes `files/notes/`. Read 2026-10-21; it is not in the repository.
- The study's sections are A–J, its questions `И1`–`И32`: this is its own numbering. The numbers of decisions in it — `R03`, `R04`, `R05`, `R09` — are spec's numbers, not this log's.
- The researchers read their sources on 2026-10-20, and their quotes are verbatim. Below is what R06 (the 2026-10-21 amendment), R08, R11 and the 2026-10-21 plan of measurements 1.4–1.7 rest on, with the primary source the study names. Apart from Google's terms and the law, sources 1.1 and sources 3.1, these primary sources were not read again here: the quotes are as the study gives them:
- **Google Cloud.** §20(f) and the archive of the terms of 2024-07-18 are in sources 1.1: they were checked here on 2026-10-21. By the study's reading, not a lawyer's, the prohibition of §20(d) closes Google's generative models to the whole platform, which is open to children (`И1`).
- **The place on the handwriting.**
  - HG-Bench — 500 marked-up homework papers of Chinese K-12 pupils, a box counting at IoU ≥ 0.5: "no zero-shot system exceeds 55.22% on FA or 48.22% on FSm" ([arXiv 2606.25491](https://arxiv.org/html/2606.25491v1)).
  - Marks instead of coordinates, Set-of-Mark ([arXiv 2310.11441](https://arxiv.org/abs/2310.11441)), were tested on natural photos, not on handwriting. And "other Multimodal Large Language Models (MLLMs) struggle to understand these visual tags" ([arXiv 2404.16375](https://arxiv.org/abs/2404.16375)).
- **The model's confidence:** "current VLMs often display notable miscalibration" ([arXiv 2505.20236](https://arxiv.org/abs/2505.20236)); "verbalized confidence is largely trajectory-independent" ([arXiv 2609.18453](https://arxiv.org/abs/2609.18453)).
- **The height of digits.** The study found no norm of how many pixels a character a model needs for a child's handwriting. The nearest is printed English: "a near-perfect Stable Phase, an Instability Phase marked by increased error variance, and a total Collapse Phase", with no numerical threshold ([arXiv 2602.02539](https://arxiv.org/abs/2602.02539)).
- **Injections:** "an average attack success rate of 82.50% on six promising open-source LVLMs" (FigStep, [arXiv 2311.05608](https://arxiv.org/abs/2311.05608)); "synthetic attacks closely resemble real-world (handwritten) attacks" (SCAM, [arXiv 2504.04893](https://arxiv.org/abs/2504.04893)).
- **The key in the prompt.** Students wrote letters in tables of answers, and with the key in the prompt the false "correct" verdicts grew at every model — for example, at GPT-5.2 from 5 to 80, at Grok-4.3 from 1 to 193 ([arXiv 2606.11477](https://arxiv.org/html/2606.11477v1)).
- **The course of reasoning.** The only controlled gain from strokes in time is at fine-tuned models on pictures of 224 and 288 px: "Combining both image and ink input yields better performance than when using only one of the input modalities" ([arXiv 2402.15307](https://arxiv.org/html/2402.15307v1)).
  - By the study's calculation from the same paper, at PaLM-E a picture alone read better than ink alone: an error by character of 4.87 against 6.46.
  - On the first mistake, the trap and the hint there are no data for any kind but the final picture (A1–A2 of the study).
- **The representations of the work** — the study's measurement on a synthetic "ladder": 24 strokes, 2 erased, 1,265 points (A1–A3):
  - a crop of 378×312 with digits of 32 px — a PNG of 2,339 B, 168 tokens at Claude and 144 at OpenAI;
  - raw strokes — 28,984 B, of the order of 10,900 text tokens;
  - compact polylines — 869–2,127 B;
  - three snapshots — 5,615 B;
  - the trace of the process — 269–337 B.
- **The scale of classes** — the study's reading, not a lawyer's (A4):
  - «Риск по возрастанию: (a) картинка < (b) картинка и грубые признаки процесса < (d) стёртое ≤ (c) сырые штрихи с временем и нажимом»;
  - «процесс картинками», ordered snapshots of one line width, by the study «держит правовой класс ближе к (a), но несёт риск ложной тревоги по исправленному» (A6);
  - whether such snapshots, and a trace of the process without time and pressure, are equal to a picture is a question for a lawyer (`И9`).
- **The frame before erasure** (A1–A2): «снимок до стирания показывает ошибку, которую ребёнок уже исправил, — разбор по нему даст ложную тревогу, а если стёрт был ключ, подсказка по нему назовёт ответ».
- **Pauses** (B1–B2):
  - 80 children of grades 3 and 5, writing letters: "The average duration of handwriting pauses was around 400ms"; "Pausing rate was around 1.8 pauses per letter"; "there is only a weak empirical basis to claim that specific average numbers and durations of pauses may be associated with specific writing processes" ([Pascual, Soler & Salas 2023](https://www.frontiersin.org/journals/psychology/articles/10.3389/fpsyg.2022.1052264/full));
  - "Do not interrupt learner with feedback if the learner is actively engaged" ([Shute 2008](https://myweb.fsu.edu/vshute/pdf/shute%202008_b.pdf)).
- **The pen's signals** tell of the learner, not of the place of the mistake:
  - "correctly classified 79-92% of students by their domain expertise level" ([Oviatt et al. 2018](https://opus.lib.uts.edu.au/handle/10453/132779));
  - on a new task the models of difficulty do not work: "all R² scores fall below zero". By the dynamics of writing, the literature detects "coordination disorder [33], dysgraphia [32] or high-functioning autism spectrum disorder [34]" ([Barz et al. 2020](https://www-live.dfki.de/fileadmin/user_upload/import/10894_digital_pen_predicts_task_performance.pdf)).

### 5.2. The analysis «Базовая канва и контракт „канва ↔ агент“»

The analysis «Базовая канва и контракт „канва ↔ агент“» ("The base canvas and the contract 'canvas ↔ agent'") — the author sent it on 2026-10-21; it is not in the repository. It was written by a model that saw neither the code nor NovaTrail's decisions, so what is taken from it is questions, not facts: the amendment to R03, О-8 in measurements 1.4–1.7 and В15 of table 3 of [RUN.md](../RUN.md), which R43 spec closed. spec names it by the same title and with the same reservation: product 14 spec, now sources 8.4 spec (R10).

### 5.3. canvas-api at commit 0901af0

[mathtrail/canvas-api](https://github.com/mathtrail/canvas-api), commit 0901af0, in a clone in a temporary folder. Read 2026-10-20, for R03; what T08 added — T77, T72 and the check by pause — on 2026-10-21:
- **The measurement was never made.** The tasks of E2 T63–T77 are not done, and there are no reports `docs/live/02–04`. In the log: «спайки S1 и S2 не проводились» (R60 canvas-api).
- R62 canvas-api: «опаснее всего ложный `CORRECT` — модель видит ключ в неверной записи».
- R66 canvas-api: «в штрихах есть то, чего нет в картинке, — порядок написания».
- T77 canvas-api, «PNG или векторы», compared three options:
  - «PNG, нарисованный на сервере из штрихов»;
  - «штрихи текстом с порядком написания, без картинки»;
  - «картинка и порядок написания вместе».

  The option without a picture did not pass the check of the picture (T69 canvas-api), and eval called the model for it «своим путём — расширением порта или отдельным входом адаптера». T77 was never done.
- The «Журнал правок» of T72 canvas-api is named, but it has no fields. There are no key frames and no log of actions either in canvas-api or in its draft `docs/plan/E2-draft.md` (read 2026-10-21, a clone in a temporary folder).
- canvas-api rejected the check by pause. R37 canvas-api: «проверка каждого шага по паузе: распознавание каждой строки и смысл шага снова требуют правил под предмет, а с задержкой модели тихая проверка превращается в ожидание». Nor is it in canvas-api: R38, R61.

### 5.4. canvas-api at revision 2796f48, 2026-10-21

The revision before the turn to reading by a model. Later these decisions were superseded by canvas-api: R37, R42, and at 0901af0 they stand in `docs/decisions.md` under «Заменено». Read 2026-10-21:
- R16 canvas-api: «Строка считается законченной, когда ребёнок перешёл к другой строке, нажал Check или 6 секунд не трогал холст. До этого ошибка в ней не показывается.» The first version «показывала ложную ошибку на недописанной строке»;
- R18 canvas-api: the strokes went to Mathpix «без времени»;
- R19 canvas-api: «выбор черновика — „цифровые чернила“ точнее картинки на детском почерке». The spike S1 (spec 20 canvas-api) was to check it, and S1 never took place (R60 canvas-api);
- product 5.2 canvas-api: «Проверка запускается после паузы 1,5–2 секунды или по кнопке **Check**»; «пауза посреди мысли — это нормально».
- T74 canvas-api: «сотня листов — разведка (≤ 5 %)», not the check of the goals of the beta (≤ 1%) of product 12 canvas-api.
- The goals of the beta in product 12 canvas-api: false alarms ≤ 1%, false corrects ≤ 0.5%, needless requests to rewrite ≤ 5%, the accuracy of the place ≥ 80%.
