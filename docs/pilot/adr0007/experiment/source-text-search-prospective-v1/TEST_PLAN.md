# Input-only discriminating cases

1 empty query fail
2 ASCII exact one
3 ASCII no match
4 case-sensitive miss
5 overlapping aba in ababa
6 adjacent aa in aaa
7 query at byte 0
8 query at EOF
9 CRLF offset
10 LF offset
11 bare CR offset
12 non-BMP before match
13 non-BMP inside query
14 combining mark literal
15 invalid UTF-8 query fail
16 source digest mismatch fail
17 path binding mismatch fail
18 revision absent fail
19 object absent fail
20 admission absent fail
21 seal absent fail
22 max matches equality pass
23 max matches plus one fail
24 max work equality pass
25 max work plus one fail
26 max output equality pass
27 max output plus one fail
28 deterministic path-byte order
29 no ranking field
30 regex metachar literal
31 fuzzy near miss rejected
32 token boundary ignored
33 duplicate source tuple preserved
34 Location RANGE_UNION preserves overlaps
35 failure counters zero
36 symlink rejected
37 path traversal rejected
38 freeze deterministic second generation equality
