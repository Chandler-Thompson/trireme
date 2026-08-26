import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(os.path.dirname(
    os.path.abspath(__file__))), "src"))

from intervals import Interval  # noqa: E402


class TestConstruction(unittest.TestCase):
    def test_endpoints_kept(self):
        i = Interval(2, 7)
        self.assertEqual((i.lo, i.hi), (2, 7))

    def test_single_point_is_legal(self):
        self.assertEqual(Interval(4, 4).length(), 1)

    def test_inverted_is_rejected(self):
        with self.assertRaises(ValueError):
            Interval(7, 2)


class TestLength(unittest.TestCase):
    def test_counts_both_endpoints(self):
        self.assertEqual(Interval(1, 3).length(), 3)

    def test_negative_range(self):
        self.assertEqual(Interval(-3, -1).length(), 3)


class TestMerge(unittest.TestCase):
    def test_spans_both(self):
        self.assertEqual(Interval(1, 3).merge(Interval(8, 9)),
                         Interval(1, 9))

    def test_is_symmetric(self):
        a, b = Interval(-2, 4), Interval(3, 11)
        self.assertEqual(a.merge(b), b.merge(a))


class TestOverlaps(unittest.TestCase):
    """Deliberately does not cover the touching case — that gap is what the
    example sortie's T-002 exists to close."""

    def test_clearly_disjoint(self):
        self.assertFalse(Interval(1, 2).overlaps(Interval(9, 10)))

    def test_clearly_nested(self):
        self.assertTrue(Interval(0, 10).overlaps(Interval(4, 6)))


if __name__ == "__main__":
    unittest.main()
