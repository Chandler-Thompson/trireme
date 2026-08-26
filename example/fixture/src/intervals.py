"""Closed integer intervals.

A deliberately small library. It exists so the example sortie has real code to
decompose against, with one real defect planted in it (see `overlaps`).
"""


class Interval:
    """The closed interval [lo, hi]. Both endpoints are members."""

    def __init__(self, lo: int, hi: int):
        if lo > hi:
            raise ValueError(f"empty interval: [{lo}, {hi}]")
        self.lo = lo
        self.hi = hi

    def __repr__(self):
        return f"Interval({self.lo}, {self.hi})"

    def __eq__(self, other):
        return (isinstance(other, Interval)
                and (self.lo, self.hi) == (other.lo, other.hi))

    def __hash__(self):
        return hash((self.lo, self.hi))

    def length(self) -> int:
        """Number of integers in the interval. [3, 3] has length 1."""
        return self.hi - self.lo + 1

    def overlaps(self, other: "Interval") -> bool:
        """True when the two intervals share at least one integer.

        DEFECT (this is the example sortie's T-002): these are CLOSED
        intervals, so [1, 3] and [3, 5] share the integer 3 and do overlap.
        The comparisons below are strict, so this returns False for them.
        """
        return self.lo < other.hi and other.lo < self.hi

    def merge(self, other: "Interval") -> "Interval":
        """The smallest interval containing both. Correct as written."""
        return Interval(min(self.lo, other.lo), max(self.hi, other.hi))
