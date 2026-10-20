# Prompts

Each P01-P16 template is one prompt variant: the poc role text, then the context its builder added.
`Render(step, data)` fills it; a key the data lacks fails with its name.

Keys: `Input` (brief and attachments), `Research`, `Personas`, `Functionalities`, `MustFunctionalities`,
`Flows`, `Constraints` (text the caller formats), `Epic` (a `pipeline.EpicOutline`), `Epics` and `Stories`
(counts, 0 for no target).

`triage` is new, not from the poc: the triage step's prompt, with `Context` (contexto.md) as its only key.

## Source and removals

Sources are in the reference snapshot c10131b, `poc/src/pipeline/`. Removed sentences are described, not quoted.

| ID | Source | Template | Removed or changed |
|---|---|---|---|
| P01 | `triage.ts:6-18, 54-56` | `p01-triage-classifier` | A named company in the complex example, now an enterprise dashboard. |
| P02 | `steps/step0-research.ts:6-15, 48-54` | `p02-research-full` | Nothing. |
| P03 | `steps/step0-research.ts:21, 56-62` | `p03-research-business` | The answer format and research instructions changed; see [research usage](../../../README.md#commands). |
| P04 | `steps/step0-research.ts:26, 56-62` | `p04-research-users` | Same. |
| P05 | `steps/step0-research.ts:31, 56-62` | `p05-research-market` | Same. |
| P06 | `steps/step0-research.ts:36, 56-62` | `p06-research-technical` | Same. |
| P07 | `steps/step0-research.ts:41, 56-62` | `p07-research-project` | Same. |
| P08 | `steps/step1-analysis.ts:6-13, 46-60` | `p08-analysis-full` | The five fixed research labels: `Research` comes formatted. |
| P09 | `steps/step1-analysis.ts:19, 62-76` | `p09-analysis-personas` | Same. |
| P10 | `steps/step1-analysis.ts:24, 62-76` | `p10-analysis-functionalities` | Personas from the preceding call follow the research. |
| P11 | `steps/step1-analysis.ts:29, 62-76` | `p11-analysis-flows` | Personas and functionalities from preceding calls follow the research. |
| P12 | `steps/step1-analysis.ts:34, 62-76` | `p12-analysis-constraints` | Previous personas, functionalities and flows follow the research; the mobile-platform example constraint was removed. |
| P13 | `steps/step1-analysis.ts:39, 62-76` | `p13-analysis-questions` | Previous personas, functionalities, flows and constraints follow the research. |
| P14 | `steps/step2-epics.ts:5-16, 76-96` | `p14-epics-full` | The issue-tracker brand the epics were likened to. |
| P15 | `steps/step2-epics.ts:18-34, 98-123` | `p15-epics-outlines` | Product-specific BAD/GOOD epic names (line 23-24), now generic; "for a solo dev iterating fast" (26); "a solo developer ... shippable app" (106), now "a shippable product"; the exact count (102), now at most `Epics`, never dropping a must functionality to fit; `dependencies` is `depends_on`. |
| P16 | `steps/step2-epics.ts:36-65, 125-166` | `p16-epics-stories` | The production line-count target and maximum (43); the product's model-API timeout example (45), now a generic one; six exclusions tied to that project: monitoring, docs, versioning, remote caching and CI, responsive layout, navigation persistence (48-53); tool names in the setup-story rule (54); the image-resize title example (60); the 50-constraint cut (160), left to the caller. The answer keys are `acceptance`, `first_test` and `functionalities`, the ids of the must functionalities a story delivers, instead of `acceptanceCriteria`, `testFirst` and `useCases`; the target adds at most `Stories`, never dropping a must functionality to fit. |
