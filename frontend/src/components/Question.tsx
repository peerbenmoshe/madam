import { Button } from "./Button.tsx";

// A question that needs an answer before going on: save without a
// temperature, an unusual temperature, "Did bleeding stop?", delete
// (docs/design-brief.md §5 and §7). It takes the Save button's place on
// the page; it is never a browser dialog. The answers are specific words
// ("Save without" / "Add a temperature"), never OK and Cancel. The answer
// that goes ahead is the primary button, on the right.

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
  return (
    <div role="group" aria-label={question} className="grid gap-2.5">
      <p className="font-bold">{question}</p>
      {note && <p className="text-sm">{note}</p>}
      <div className="grid grid-cols-2 gap-2">
        <Button variant="secondary" onClick={onNo}>
          {no}
        </Button>
        <Button size="regular" onClick={onYes}>
          {yes}
        </Button>
      </div>
    </div>
  );
}
