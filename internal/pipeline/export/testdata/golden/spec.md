# Spec

## Research

### users

Second-hand bike buyers want to see a bike before they pay for it.

Sources:

- [Bike buyers survey](https://example.com/bike-buyers): buyers want to see a bike before they pay

## Personas

- Rosa (seller)
- Tom (buyer)

## Functionalities

- F1 [must] List a bike: Only red-74 bikes
- F2 [could] Search: Find bikes by size
- F3 [must] Pay: Card at pickup

## Flows

- Listing: Scan the tag → Publish

## Constraints

- Only bicycles tagged red-74 can be listed

## Open questions

- Who prints the red-74 tags?

## Epics

### E1 List a red-74 bike end-to-end

A seller lists a tagged bike.

Depends on: none.

- #1 US1 Reject a bike without a red-74 tag
- #2 US2 Publish a red-74 bike

### E2 Pay at pickup

A buyer pays when collecting the bike.

Depends on: E1.

- #3 US3 Charge the card at pickup
