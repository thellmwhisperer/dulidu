# Brief de diseño

## Home

A member sees this week's free hours on each wheel and books one.

```
┌──────────────────────────────┐
│ Clay Corner          [Menu]  │
├──────────────────────────────┤
│ Tuesday 21                   │
│ ┌──────────┐ ┌──────────┐    │
│ │ 19:00    │ │ 20:00    │    │
│ │ Wheel 2  │ │ Wheel 1  │    │
│ │ [Book]   │ │ [Book]   │    │
│ └──────────┘ └──────────┘    │
├──────────────────────────────┤
│ [Home]          [History]    │
└──────────────────────────────┘
```

**Components:**

- Day header with the date
- Free-hour cards, each with its hour, wheel and Book button

**Navigation:**

It opens on launch and from the Home tab. Booking an hour stays here with the card marked booked; the History tab goes to History.

**States:**

- Empty: no free hour this week, with a line saying so
- Loading: card outlines while the hours load
- Error: the hours could not load, with a Retry button

**Accessibility:**

A booked card says "Booked" in text and shows a tick, not only a different fill; every button has a text label.

## History

A member sees the hours they booked and the pieces they dropped off for the kiln.

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

**Components:**

- Bookings list, newest first
- Pieces list with each piece's firing status

**Navigation:**

The History tab opens it; the Home tab goes back to Home.

**States:**

- Empty: no booking and no piece yet, with a link to Home
- Loading: list outlines while the history loads
- Error: the history could not load, with a Retry button

**Accessibility:**

Each piece's status is written out in words, never shown by colour alone; list rows read as one sentence to a screen reader.
