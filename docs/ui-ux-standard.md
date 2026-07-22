# SKYACS UI/UX Standard

This document turns the project-wide [non-generic UI/UX rules](https://raw.githubusercontent.com/wweagle09/ui-ux-design/refs/heads/main/rules-design.md) into implementation requirements for SKYACS. It is the review baseline for every console change.

## Product context

SKYACS is an operational TR-069/CWMP control plane, not a general-purpose SaaS dashboard.

- **Primary user:** network operator responsible for CPE availability, diagnosis, and configuration.
- **Secondary users:** read-only support staff and auditors; full-access administrators responsible for operators, admission controls, and provisioning rules.
- **Primary goal:** identify CPE state and complete a safe remote-management task with verifiable feedback.
- **Frequent tasks:** find a CPE, inspect freshness and faults, summon a CWMP session, review parameters, and track queued tasks.
- **Highest-risk tasks:** factory reset, firmware delivery, credential changes, bulk summons, device deletion, and security-policy changes.
- **Required data:** serial number, manufacturer, product class, IP address, model, software version, Inform time, CWMP state, parameters, faults, and task state.
- **Sensitive data:** operator credentials, CPE credentials, PPPoE secrets, Wi-Fi keys, connection-request secrets, firmware tokens, and audit source addresses.
- **Permissions:** full-access operators may mutate state; read-only operators receive the same operational context without mutation controls or sensitive values.
- **Primary failure modes:** API outage, expired session, offline CPE, connection-request rejection, partial telemetry, task failure, upload rejection, permission denial, and stale data.
- **Device priorities:** information-dense desktop and laptop use first; complete task and recovery support on mobile; no hover-only behavior.
- **Accessibility:** WCAG 2.2 AA contrast target, semantic controls, visible focus, keyboard-complete flows, labelled inputs, named icon controls, focus-contained dialogs, reduced motion, and 44 px mobile targets where practical.
- **Brand personality:** precise, restrained, technical, dependable, and operationally dense. The visual signature is IBM Plex typography, tabular telemetry, square control geometry, and network-domain language.
- **Constraints:** one control-plane instance, variable CPE vendor support, asynchronous CWMP sessions, potentially wide parameter trees, and sensitive infrastructure data.

## Information architecture

Primary navigation follows an operator's mental model:

1. **Fleet status** — decide whether the managed fleet needs attention.
2. **CPE inventory** — locate a device and enter its operational record.
3. **CWMP faults** — resolve protocol and parameter failures.
4. **Firmware library** — manage deployable device images.
5. **Access controls** — review security events and blocked CPE identities.
6. **System settings** — manage operators, provisioning rules, and connection-request configuration.

The CPE detail route is entered from the inventory and remains outside primary navigation because it represents a selected device, not a fleet-level destination.

## Screen specifications

| Screen | Purpose and primary task | Required states and recovery | Responsive, keyboard, and permission behavior |
| --- | --- | --- | --- |
| Fleet status | Help an operator decide whether offline CPEs, faults, or stale telemetry need action. Primary action: refresh fleet data. | Skeleton matching the telemetry register, explicit API error with retry, partial-analytics notice, zero-CPE guidance, and last-updated time. | Metrics preserve priority on small screens. All actions are buttons. Mutation controls are absent. |
| CPE inventory | Find a CPE by serial, vendor, model, or address and open its record. | Search-specific empty state, zero-inventory state, request error with retry, loading rows, and partial-success feedback for bulk summon. | Serial identifier remains fixed during horizontal scroll; column visibility and ordering are keyboard-operable. Summon All is hidden for read-only users. |
| CPE detail | Diagnose one device and queue remote operations. | Record error, partial parameter/task error, predictable skeleton, persistent actionable operation errors, expandable technical details, and explicit queued/success state. | Wide tables keep the identifier visible. Destructive actions use focus-trapped confirmations. Sensitive values and mutation controls respect role. |
| CWMP faults | Filter active or resolved faults and resolve or delete a record. | Loading rows, filter-aware empty state, request error with retry, pending disabled state, and operation feedback with retained context. | Visible row actions have accessible names and 44 px mobile targets. Full-access actions are hidden from read-only operators. |
| Firmware library | Upload a validated firmware image and review available versions. | Inline file/version validation, preserved form values after failure, upload progress state, request error with retry, and empty library guidance. | Form fields remain labelled and reachable above the mobile keyboard. Delete requires a consequence-aware confirmation and full access. |
| Access controls | Review audit evidence and unblock a CPE identity. | Independent loading and error states, specific audit/blocklist empty states, refresh, local input validation, and unblock confirmation. | Tables retain identifiers during horizontal scroll. Security mutations require full access; evidence remains readable without color. |
| System settings | Configure operators, provisioning, and connection-request policy. | Section-level loading/error/empty states, inline validation, preserved inputs, confirmation for destructive changes, and explicit save feedback. | Dialogs trap and restore focus. Long forms scroll within the viewport. Read-only operators see configuration context without mutation actions. |
| Sign in | Establish an audited operator session. | Rate-limited authentication feedback, retained username, visible field association, disabled/loading submission, and session-expiry redirect. | Autofill-compatible fields, keyboard submission, named password reveal control, and a single-column mobile layout. |

Analytics tracking is intentionally not embedded in the console. Security audit events are the authoritative record for state-changing operator actions; future product analytics require a separate privacy and retention decision.

## Foundations and tokens

- **Typography:** IBM Plex Sans for interface copy; IBM Plex Mono for serials, addresses, parameter paths, timestamps where alignment matters, and code-like values. Display `20 px`, heading `16 px`, section heading `13 px`, body `14 px`, label `11–12 px`, caption `10–11 px`, and data `10–24 px` according to density.
- **Color:** `action-primary`, `accent`, `success`, `warning`, `error`, neutral text, surface, border, disabled, focus, and selection are semantic CSS variables. Color never carries status alone; pair it with text or an icon.
- **Shape:** 3 px control radius and 2–3 px panel radius. Rounded marketing-card geometry, glow, glass, decorative gradients, and purple-family default accents are prohibited.
- **Density:** compact by default because operators compare many records. Spacing communicates grouping; not every block becomes a card.
- **Motion:** fast interaction and standard transition tokens only. Motion explains state change, respects `prefers-reduced-motion`, and never hides waiting time.
- **Layering:** base content, sticky navigation/header, mobile scrim, dialogs, then system notifications. New z-index values require a documented layer need.

## Reusable component contracts

### Dialog

- **Purpose:** confirmations and short, focused edit forms.
- **Inputs:** title, optional description, small/medium/large size, close behavior, content, and actions.
- **Behavior:** closes on Escape and optional backdrop activation; traps Tab/Shift+Tab; restores prior focus; prevents page scroll.
- **Responsive:** centered on larger screens and bottom-aligned within mobile safe areas.
- **Usage:** use for destructive confirmation or a short form. Do not use for parameter trees or other long workflows.

### Feedback

- **Purpose:** communicate success, partial success, information, or a recoverable operation failure.
- **Variants:** success, information, and persistent error; technical detail is expandable.
- **Behavior:** success/information dismiss after five seconds; errors remain until dismissed. Messages identify what happened and the next action.
- **Usage:** do not use a notification for information requiring a decision; use a dialog or inline alert instead.

### Resource state

- **Purpose:** predictable empty and error presentation inside data regions.
- **Inputs:** specific title, explanation, and optional real recovery action.
- **Behavior:** errors use an alert role and expose retry when retry is safe. Empty states explain what is missing, why, and what can happen next.
- **Usage:** never use generic “No data” or “Something went wrong” copy.

### Data table

- **Purpose:** structured comparison of operational records.
- **Behavior:** visible labels and units, consistent formatting, loading/error/empty states, and fixed first identifier on narrow screens.
- **Responsive:** hide low-priority columns where appropriate; otherwise allow deliberate horizontal scroll with a fixed identifier. Essential actions remain visible.
- **Accessibility:** real links for navigation, buttons for actions and sorting, `aria-sort` for sortable columns, and accessible names for icon-only controls.

### Page header and operational register

- **Purpose:** state the domain task and surface its highest-priority action or decision data.
- **Behavior:** titles use concrete domain terms. Registers show only metrics tied to an operator decision and include data freshness.
- **Usage:** avoid generic titles, vanity statistics, repeated equal-weight cards, and charts without an operational question.

## Content and interaction rules

- Use `CPE`, `CWMP`, `Inform`, `parameter`, `fault`, `task`, `provisioning rule`, and `connection request` consistently.
- State whether an operation is queued, processing, complete, partially complete, or rejected.
- Preserve entered values after validation or network failure.
- Explain destructive consequences before confirmation. Factory reset additionally requires password re-authentication.
- Do not expose secrets to read-only roles. Technical errors appear only behind an expandable disclosure.
- Never use vague calls to action when the operation can be named precisely.

## Definition of done

A UI change is complete only when:

- Its user, task, risk, permissions, failure cases, and information priority are understood.
- Default, loading, empty, error, success, permission, partial-data, long-content, and narrow-screen states have been evaluated.
- Keyboard navigation, focus return, screen-reader names, label associations, zoom, contrast, reduced motion, and mobile touch targets are verified.
- Existing semantic tokens and components are used; new reusable patterns are documented here.
- Copy is concrete, domain-specific, and actionable.
- Realistic long serials, parameter paths, errors, empty results, and large tables have been exercised.
- `npm run check:ui` and `npm run build` pass.

The automated gate intentionally covers objective regressions only. Product-task fit, responsive prioritization, copy quality, and realistic-data review still require human judgment.
