# Spec

## Research

### users

Studio members want a wheel free when they arrive and to know their pieces made the weekly firing.

Sources:

- [Studio members survey](https://example.com/studio-members): members want to know their pieces made the firing

### market

Community studios book wheels on paper sign-up sheets or general booking apps that know nothing of kilns.

Sources:

- [Tools small studios use](https://example.com/studio-tools): studios book wheels on paper sheets

## Personas

- Ana (member). Goals: Throw on a wheel after work. Pain points: Every wheel taken when she arrives
- Luis (studio manager). Goals: Fill Friday's kiln. Pain points: Pieces dropped off after the firing is loaded

## Functionalities

- F1 [must] Book a wheel: One hour, one member per wheel
- F2 [must] Drop off for the kiln: Pieces in before Friday 6 pm make that week's firing
- F3 [could] Waiting list: Told when a wheel frees up

## Flows

- Booking: Pick a free hour → Pick a wheel → Confirm
- Firing: Drop a piece off → Kiln fires on Friday → Collect

## Constraints

- The kiln fires once a week, on Friday at 6 pm
- Bookings are free: members pay a monthly fee

## Open questions

- How many wheels does the studio have?

## Epics

### E1 Book a wheel

A member books a wheel for an hour.

Depends on: none.

- #1 US1 Book a free wheel for an hour
- #2 US2 Refuse a wheel already booked

### E2 Make the weekly firing

A member's pieces make Friday's kiln.

Depends on: E1.

- #3 US3 Drop a piece off before the firing
