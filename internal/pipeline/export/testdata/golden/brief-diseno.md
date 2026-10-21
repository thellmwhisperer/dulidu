# Brief de diseño

## Listing

Rosa lists a red-74 bike by scanning its tag.

```
┌────────────┐
│ Scan a tag │
│ [Publish]  │
└────────────┘
```

**Components:**

- Tag scanner
- Publish button

**Navigation:**

From the home tab; Publish goes to the bike's page.

**States:**

- Empty: no tag scanned yet
- Refused: the tag is not red-74

**Accessibility:**

A refused tag says so in words, not only in red.
