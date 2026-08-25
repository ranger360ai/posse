# At-least-once delivery and idempotent handlers

## The concept

Over an unreliable channel, a sender cannot know whether the receiver
acted — Gray's Generals Paradox: "no fixed length protocol exists" that
lets two parties reach certain agreement over lossy messengers (Gray
1978, §5.8.3.3.1). So every delivery guarantee is a choice of failure:
retry until acknowledged (**at-least-once**: duplicates possible) or
never retry (**at-most-once**: loss possible). **Exactly-once
*delivery* is not on the menu.** What products sell as "exactly-once"
is exactly-once *processing* — at-least-once delivery plus
deduplication or transactions on the receiving side (Kafka: idempotent
producer + transactional offsets; Confluent's own framing).

The **visibility timeout** is at-least-once's standard clock: a
consumed message "remains in the queue but becomes temporarily
invisible"; if not deleted before the timeout expires, it "becomes
visible again … and can be retrieved by another consumer" (AWS SQS
docs). Read that carefully: **the timeout expiring is a statement about
the consumer's silence, not about the work's failure** — the consumer
may be slow, paused, or finished-but-unacknowledged. Redelivery after
timeout is how duplicates are born.

The standard answer on the receiving side is **idempotence**: "the
receiving application must make handling retried messages harmless"
(Helland's canonical treatment, *Idempotence Is Not a Medical
Condition*). Practical forms: natural idempotence (set-to-value, not
increment), **idempotency keys** chosen by the sender so the receiver
can deduplicate retries of the same logical operation (Stripe's API
pattern), or dedup state at the boundary.

## Standard rebuttals

- *"We'll just not retry."* That is at-most-once — you chose loss. Fine
  if you can say so out loud in the design.
- *"The timeout means it failed."* It means you stopped hearing. Acting
  on it as a verdict (unclaim, redispatch, delete) manufactures the
  duplicate the timeout was warning you about.
- *"We deduplicate by message id."* Dedup state must outlive the
  retry horizon and survive the receiver's restarts, or it is a
  snapshot (`toctou.md`).

## How posse applies it

A dispatch `--wait` timeout **is** a visibility timeout, and the rule is
this file applied: *a `--wait` timeout is a check-in, not a
verdict* — the gather step re-asks the agent's state, and **a timeout
never unclaims**. Prompting an agent is not idempotent (a double-prompt
derails a live session), so dedup lives at the prompt boundary: the
durable claim, plus a prompt grace that a settled session is "stopped on
purpose" and not re-prompted every pass. ADR 0011 §3 is that dedup state
being persisted into the session's run record rather than a process's
memory — because dedup state that does not outlive the retry horizon and
the receiver's restarts is just another snapshot.

## Sources (verified 2026-08-20)

- **[paper]** Gray, "Notes on Data Base Operating Systems", 1978
  (Generals Paradox §5.8.3.3.1) —
  https://jimgray.azurewebsites.net/papers/dbos.pdf
- **[paper]** Helland, "Idempotence Is Not a Medical Condition", ACM
  Queue 10(4), 2012 — https://queue.acm.org/detail.cfm?id=2187821
  (record: DOI 10.1145/2181796.2187821; ACM bot-blocks fetches)
- **[docs]** AWS, "Amazon SQS visibility timeout" —
  https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-visibility-timeout.html
- **[docs]** Stripe, "Idempotent requests" —
  https://docs.stripe.com/api/idempotent_requests (companion essay:
  https://stripe.com/blog/idempotency)
- **[blog]** Treat, "You Cannot Have Exactly-Once Delivery", 2015 —
  https://bravenewgeek.com/you-cannot-have-exactly-once-delivery/
- **[blog]** Narkhede, "Exactly-Once Semantics Are Possible: Here's How
  Kafka Does It", Confluent 2017 (the processing-not-delivery
  counterpoint) —
  https://www.confluent.io/blog/exactly-once-semantics-are-possible-heres-how-apache-kafka-does-it/
