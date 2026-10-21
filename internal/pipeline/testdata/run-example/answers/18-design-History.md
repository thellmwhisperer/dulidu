## History
### Wireframe
```
┌──────────────────────────────┐
│ History                      │
├──────────────────────────────┤
│ Bookings                     │
│ • Tue 21  19:00  Wheel 2     │
│ • Sat 25  10:00  Wheel 1     │
│ Pieces                       │
│ • Bowl    In Friday's firing │
│ • Mug     Ready to collect   │
├──────────────────────────────┤
│ [Home]          [History]    │
└──────────────────────────────┘
```
### Components
- Bookings list, newest first
- Pieces list with each piece's firing status
### Navigation
The History tab opens it; the Home tab goes back to Home.
### States
- Empty: no booking and no piece yet, with a link to Home
- Loading: list outlines while the history loads
- Error: the history could not load, with a Retry button
### Accessibility
Each piece's status is written out in words, never shown by colour alone; list rows read as one sentence to a screen reader.
