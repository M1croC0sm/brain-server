package signals

// Stopwords is a set of common words to exclude from term extraction
var Stopwords = map[string]bool{
	// Articles
	"a": true, "an": true, "the": true,

	// Pronouns
	"i": true, "me": true, "my": true, "myself": true,
	"you": true, "your": true, "yours": true, "yourself": true,
	"he": true, "him": true, "his": true, "himself": true,
	"she": true, "her": true, "hers": true, "herself": true,
	"it": true, "its": true, "itself": true,
	"we": true, "us": true, "our": true, "ours": true, "ourselves": true,
	"they": true, "them": true, "their": true, "theirs": true, "themselves": true,
	"this": true, "that": true, "these": true, "those": true,
	"what": true, "which": true, "who": true, "whom": true,

	// Be verbs
	"am": true, "is": true, "are": true, "was": true, "were": true,
	"be": true, "been": true, "being": true,

	// Have verbs
	"have": true, "has": true, "had": true, "having": true,

	// Do verbs
	"do": true, "does": true, "did": true, "doing": true, "done": true,

	// Modal verbs
	"will": true, "would": true, "shall": true, "should": true,
	"can": true, "could": true, "may": true, "might": true, "must": true,

	// Common verbs
	"get": true, "got": true, "getting": true,
	"go": true, "goes": true, "going": true, "went": true, "gone": true,
	"make": true, "made": true, "making": true,
	"take": true, "took": true, "taken": true, "taking": true,
	"come": true, "came": true, "coming": true,
	"see": true, "saw": true, "seen": true, "seeing": true,
	"know": true, "knew": true, "known": true, "knowing": true,
	"think": true, "thought": true, "thinking": true,
	"want": true, "wanted": true, "wanting": true,
	"need": true, "needed": true, "needing": true,
	"try": true, "tried": true, "trying": true,
	"use": true, "used": true, "using": true,
	"find": true, "found": true, "finding": true,
	"give": true, "gave": true, "given": true, "giving": true,
	"tell": true, "told": true, "telling": true,
	"say": true, "said": true, "saying": true,
	"let": true, "lets": true, "letting": true,
	"put": true, "puts": true, "putting": true,
	"keep": true, "kept": true, "keeping": true,
	"start": true, "started": true, "starting": true,
	"seem": true, "seemed": true, "seeming": true,
	"help": true, "helped": true, "helping": true,
	"show": true, "showed": true, "shown": true, "showing": true,
	"feel": true, "felt": true, "feeling": true,
	"look": true, "looked": true, "looking": true,

	// Prepositions
	"to": true, "of": true, "in": true, "for": true, "on": true,
	"with": true, "at": true, "by": true, "from": true, "up": true,
	"about": true, "into": true, "over": true, "after": true, "before": true,
	"between": true, "under": true, "again": true, "out": true, "off": true,
	"down": true, "through": true, "during": true, "without": true,
	"around": true, "among": true, "along": true, "across": true,

	// Conjunctions
	"and": true, "but": true, "or": true, "nor": true, "so": true,
	"yet": true, "both": true, "either": true, "neither": true,
	"not": true, "only": true, "also": true, "just": true,
	"than": true, "then": true, "when": true, "where": true, "why": true,
	"how": true, "if": true, "because": true, "while": true, "although": true,
	"though": true, "unless": true, "until": true, "whether": true,

	// Determiners and quantifiers
	"all": true, "each": true, "every": true, "any": true, "some": true,
	"no": true, "none": true, "few": true, "many": true, "much": true,
	"more": true, "most": true, "less": true, "least": true,
	"other": true, "another": true, "such": true, "same": true,

	// Adverbs
	"very": true, "really": true, "quite": true, "too": true,
	"always": true, "never": true, "often": true, "sometimes": true,
	"usually": true, "already": true, "still": true, "even": true,
	"now": true, "here": true, "there": true,
	"today": true, "tomorrow": true, "yesterday": true,
	"well": true, "back": true, "way": true,

	// Other common words
	"yes": true, "ok": true, "okay": true,
	"like": true, "thing": true, "things": true,
	"time": true, "day": true, "days": true, "week": true, "weeks": true,
	"year": true, "years": true, "month": true, "months": true,
	"people": true, "person": true, "man": true, "woman": true,
	"first": true, "last": true, "next": true, "new": true, "old": true,
	"good": true, "great": true, "bad": true, "little": true, "big": true,
	"long": true, "right": true, "left": true, "own": true, "part": true,
	"lot": true, "something": true, "nothing": true, "everything": true,
	"anything": true, "someone": true, "anyone": true, "everyone": true,
	"maybe": true, "probably": true, "actually": true, "basically": true,

	// Single letters and numbers as words
	"s": true, "t": true, "m": true, "d": true, "ll": true, "ve": true, "re": true,
}

// IsStopword returns true if the word is a stopword
func IsStopword(word string) bool {
	return Stopwords[word]
}
