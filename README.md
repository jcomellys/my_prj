# Literature Review Extractor

Structured extraction of academic papers for systematic literature reviews.

Converts Claude into a **research extraction engine** that produces consistent, comparable data across papers — ready for a comparison table or thesis.

## Quick start

```bash
# Print the system prompt (paste into Claude as first message)
python -m literature_review_extractor prompt

# After analyzing papers in Claude, save each extraction
python -m literature_review_extractor add session.json --name "AuthorYear" --file extraction1.txt

# View consolidated comparison table
python -m literature_review_extractor table session.json

# Export as CSV
python -m literature_review_extractor table session.json --csv output.csv

# Generate a synthesis prompt (paste into Claude)
python -m literature_review_extractor synthesize session.json

# Generate a related-paper expansion prompt
python -m literature_review_extractor expand session.json
```

## Workflow

1. Run `python -m literature_review_extractor prompt` and paste the output as a system prompt in Claude.
2. Upload a paper PDF and type: `Analyze this paper.`
3. Copy the extraction output and save it via the `add` command.
4. Repeat for each paper.
5. Use `table` to get a consolidated Markdown/CSV comparison.
6. Use `synthesize` to generate a cross-paper synthesis.
7. Use `expand` to discover related papers you may have missed.

## Extraction steps

Each paper produces 8 structured sections:

| Step | Content |
|------|---------|
| 1 | Metadata (title, authors, year, venue, DOI, domain, study type) |
| 2 | Problem and contribution |
| 3 | Data characteristics (source, sensors, sampling, preprocessing) |
| 4 | Method (algorithm, training, validation, baselines) |
| 5 | Results (metrics, numerical values, feasibility) |
| 6 | Limitations (author-declared + critical analysis) |
| 7 | Relation to other work (5 suggested papers) |
| 8 | Single table row for consolidated comparison |
