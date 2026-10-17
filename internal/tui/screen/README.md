# Screen

Reference snapshot commit 7687e5c, taken as it was the day that snapshot was cut.

Gross changed lines are 2248, above 2000 (expected 1200 to 1800, most of it the ported package) because view.go (661), model.go (442), parse.go (306) and poll.go (190) are kept whole from that snapshot, plus screen_test.go (311). Cutting them to fit the band would drop layout, markdown, tool folding, scroll and the poll loop the tests cover. Pinning libraries does not remove source lines.
