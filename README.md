dev notes:
- counting period must be X times the sync interval. This is to make sure that the sync goroutine cleans up the old slots
- slot size must be bigger than sync interval (i.e., sync'ing multiple times within the same slot will lead to bad results... it's ok because it doesn't make sense in practice)
