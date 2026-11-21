import json
import unittest

from caller_agent import Capabilities, Service, UnsupportedHost, bind_actors, select_mode


class CallerAgentTests(unittest.TestCase):
    def setUp(self):
        self.now = 100
        self.binding = dict(ApplicationSHA256="app", RunID="run", ConfigSHA256="config",
                            Grant="grant", SourceRevision="revision")
        self.actors = bind_actors("SAME_THREAD", "observed-session")
        self.prompts = dict(interpreter="compiled-interpreter", reviewer="compiled-reviewer")
        self.calls = []

    def service(self):
        return Service(self.binding, self.actors, self.prompts, 1000, 100,
                       clock=lambda: self.now)

    def frame(self, operation, sequence, **changes):
        role = "reviewer" if operation == "Reviewer" else "interpreter"
        frame = dict(self.binding, Protocol="PRIVATE_APPLICATION_INVENTORY_V3",
                     Operation=operation, Role=role, LaunchID=self.actors[role],
                     PromptSHA256=self.prompts[role], Sequence=sequence,
                     DeadlineUnixNano=900, Payload={"untrusted": "source"})
        frame.update(changes)
        return json.dumps(frame).encode()

    def author(self, role, raw):
        self.calls.append(("author", role, json.loads(raw)["Operation"]))
        return b'{"scripted_test_only":true}\n'

    def submit(self, role, sequence, raw):
        self.calls.append(("submit", role, sequence, raw))

    def test_mode_and_missing_host(self):
        self.assertEqual(select_mode(Capabilities(True, True)), "SAME_THREAD")
        self.assertEqual(select_mode(Capabilities(True, True, True, True, True)), "SUBAGENTS")
        for partial in [(True, False, True), (False, True, True), (True, True, False)]:
            self.assertEqual(select_mode(Capabilities(True, True, *partial)), "SAME_THREAD")
        for missing in [Capabilities(False, True), Capabilities(True, False), Capabilities(False, False)]:
            with self.assertRaises(UnsupportedHost):
                select_mode(missing)

    def test_actor_binding(self):
        self.assertEqual(self.actors, {"interpreter": "observed-session", "reviewer": "observed-session"})
        ids = {"interpreter": "returned-i", "reviewer": "returned-r"}
        self.assertEqual(bind_actors("SUBAGENTS", None, ids), ids)
        for mode, caller, actors in [("SAME_THREAD", "", None), ("SUBAGENTS", "x", None),
                                      ("SUBAGENTS", None, self.actors), ("SAME_THREAD", "x", ids)]:
            with self.assertRaises(ValueError):
                bind_actors(mode, caller, actors)

    def test_separate_review_and_exact_bytes(self):
        service = self.service()
        for sequence, operation in enumerate(["Describe", "Describe", "GroupDecision", "Reviewer"], 1):
            raw = self.frame(operation, sequence)
            def author(role, received):
                self.assertEqual(received, raw)
                return self.author(role, received)
            self.assertEqual(service.service(raw, author, self.submit), operation)
        self.assertEqual([c[2] for c in self.calls if c[0] == "author"],
                         ["Describe", "Describe", "GroupDecision", "Reviewer"])
        self.assertTrue(all(c[3] == b'{"scripted_test_only":true}\n' for c in self.calls if c[0] == "submit"))
        with self.assertRaisesRegex(ValueError, "spent"):
            service.service(self.frame("Reviewer", 5), self.author, self.submit)

    def test_premature_review(self):
        service = self.service()
        with self.assertRaises(ValueError):
            service.service(self.frame("Reviewer", 1), self.author, self.submit)
        self.assertEqual(self.calls, [])

    def test_foreign_frames_and_order(self):
        alterations = [dict(RunID="foreign"), dict(LaunchID="foreign"), dict(Role="reviewer"),
                       dict(PromptSHA256="foreign"), dict(Grant="foreign"), dict(Protocol="foreign"),
                       dict(ConfigSHA256="foreign"), dict(SourceRevision="foreign"),
                       dict(ApplicationSHA256="foreign"), dict(Sequence=2), dict(Sequence=True)]
        for alteration in alterations:
            with self.subTest(alteration=alteration), self.assertRaises(ValueError):
                self.service().service(self.frame("Describe", 1, **alteration), self.author, self.submit)
        self.assertEqual(self.calls, [])

    def test_duplicate_and_postgroup_describe(self):
        service = self.service()
        service.service(self.frame("GroupDecision", 1), self.author, self.submit)
        with self.assertRaises(ValueError):
            service.service(self.frame("Describe", 2), self.author, self.submit)
        service = self.service()
        service.service(self.frame("Describe", 1), self.author, self.submit)
        with self.assertRaises(ValueError):
            service.service(self.frame("Describe", 1), self.author, self.submit)

    def test_uncertain_submit_is_spent(self):
        service = self.service()
        def uncertain(*args):
            raise OSError("uncertain external effect")
        with self.assertRaises(OSError):
            service.service(self.frame("GroupDecision", 1), self.author, uncertain)
        with self.assertRaisesRegex(ValueError, "spent"):
            service.service(self.frame("Reviewer", 2), self.author, self.submit)
        self.assertFalse(service.grouped)

    def test_deadline_before_and_after_authoring(self):
        service = self.service()
        self.now = 900
        with self.assertRaises(ValueError):
            service.service(self.frame("Describe", 1), self.author, self.submit)
        self.now = 100
        service = self.service()
        def slow(role, raw):
            self.now = 1000
            return b'{}\n'
        with self.assertRaises(ValueError):
            service.service(self.frame("Describe", 1, DeadlineUnixNano=2000), slow, self.submit)
        self.assertEqual(self.calls, [])

    def test_automatic_service_loop(self):
        operations = iter(["Describe", "GroupDecision", "Reviewer", None])
        waits = []
        def wait(role, after, deadline):
            waits.append((role, after, deadline))
            operation = next(operations)
            return dict(RunID="run", ApplicationSHA256="app",
                        Event="TERMINAL" if operation is None else "NEEDS_HOST",
                        GlobalSequence=after + 1,
                        Bundle={"operation": operation, "sequence": after + 1})
        def verified(bundle):
            return self.frame(bundle["operation"], bundle["sequence"])
        verified_result = object()  # sentinel, not a fabricated publication DTO
        service = self.service()
        result = service.run(wait, verified, self.author, self.submit, lambda event: verified_result)
        self.assertIs(result, verified_result)
        self.assertEqual(waits, [("interpreter", 0, 1000), ("interpreter", 1, 1000),
                                 ("reviewer", 2, 1000), ("reviewer", 3, 1000)])
        with self.assertRaisesRegex(ValueError, "spent"):
            service.run(wait, verified, self.author, self.submit, lambda event: verified_result)
        self.assertEqual(len(waits), 4)

    def test_loop_refuses_events_and_readback_errors(self):
        for event in [dict(Event="DEADLINE", RunID="run", ApplicationSHA256="app"),
                      dict(Event="TERMINAL", RunID="run", ApplicationSHA256="app"),
                      dict(Event="NEEDS_HOST", RunID="foreign", ApplicationSHA256="app")]:
            with self.subTest(event=event), self.assertRaises(ValueError):
                self.service().run(lambda *args: event, lambda b: self.fail("no bundle read"),
                                   self.author, self.submit, lambda e: self.fail("no readback"))
        operations = iter(["GroupDecision", "Reviewer", None])
        def wait(role, after, deadline):
            op = next(operations)
            return dict(Event="TERMINAL" if op is None else "NEEDS_HOST", RunID="run",
                        ApplicationSHA256="app", GlobalSequence=after + 1,
                        Bundle={"operation": op, "sequence": after + 1})
        def reject_readback(event):
            raise ValueError("actual verifier refused")
        service = self.service()
        with self.assertRaisesRegex(ValueError, "actual verifier refused"):
            service.run(wait, lambda b: self.frame(b["operation"], b["sequence"]),
                        self.author, self.submit, reject_readback)
        self.assertTrue(service.retired)

    def test_response_bound_and_json(self):
        for response in [b'{}', b'{}\n{}\n', b'{"x":1,"x":2}\n', b'[]\n', b'{"x":NaN}\n',
                         b'\xff\n', b'{"x":"' + b'x' * 100 + b'"}\n']:
            with self.subTest(response=response), self.assertRaises((ValueError, UnicodeError)):
                self.service().service(self.frame("Describe", 1), lambda *args: response, self.submit)
        self.assertEqual(self.calls, [])


if __name__ == "__main__":
    unittest.main(verbosity=2)
