import { useEffect, useRef } from "react";
import { Button } from "./Button.tsx";

// A question that needs an answer before going on: save without a
// temperature, an unusual temperature, "Did bleeding stop?", delete
// (docs/design-brief.md §5 and §7). It takes the Save button's place on
// the page; it is never a browser dialog. The answers are specific words
// ("Save without" / "Add a temperature"), never OK and Cancel. The answer
// that goes ahead is the primary button, on the right.
//
// When it appears, the focus moves to the answer that changes nothing.
// What had the focus (Save) is gone, or covered by the taller bar (Delete,
// just above it), and a screen reader then reads the question out. Two
// presses of Enter can't go ahead by accident.

type Props = {
  question: string;
  // Optional extra line under the question.
  note?: string;
  yes: string;
  no: string;
  onYes: () => void;
  onNo: () => void;
};

export function Question({ question, note, yes, no, onYes, onNo }: Props) {
  const noRef = useRef<HTMLButtonElement>(null);
  // Again for a new question in the same place.
  useEffect(() => {
    noRef.current?.focus();
  }, [question]);

  return (
    <div role="group" aria-label={question} className="grid gap-2.5">
      <p className="font-bold">{question}</p>
      {note && <p className="text-sm">{note}</p>}
      <div className="grid grid-cols-2 gap-2">
        <Button ref={noRef} variant="secondary" onClick={onNo}>
          {no}
        </Button>
        <Button size="regular" onClick={onYes}>
          {yes}
        </Button>
      </div>
    </div>
  );
}
