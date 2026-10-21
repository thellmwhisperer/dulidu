## Home
### Wireframe
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
### Components
- Day header with the date
- Free-hour cards, each with its hour, wheel and Book button
### Navigation
It opens on launch and from the Home tab. Booking an hour stays here with the card marked booked; the History tab goes to History.
### States
- Empty: no free hour this week, with a line saying so
- Loading: card outlines while the hours load
- Error: the hours could not load, with a Retry button
### Accessibility
A booked card says "Booked" in text and shows a tick, not only a different fill; every button has a text label.
