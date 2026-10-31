export interface ExampleSentence {
  text: string;
  ref?: string;
}

export interface Sense {
  sense_index: number;
  definition: string;
  labels?: string[];
  examples?: ExampleSentence[];
  source: string;
}

export interface Relation {
  sense_index?: number;
  type: string;
  word: string;
  qualifier?: string;
  source: string;
}

export interface Translation {
  sense_index?: number;
  sense_gloss?: string;
  lang_code: string;
  lang_name?: string;
  word: string;
  qualifier?: string;
  from_lemma?: string;
  source: string;
}

export interface CaseRow {
  case: string;
  case_name: string;
  singular?: string[];
  plural?: string[];
}

export interface ConjugationTableData {
  infinitive?: string[];
  present_or_future?: Record<string, string[]>;
  imperative?: Record<string, string[]>;
  past_participle?: string[];
  passive_participle?: string[];
  transgressive?: string[];
}

export interface InflectionSummary {
  declension?: CaseRow[];
  conjugation?: ConjugationTableData;
  degrees?: Record<string, string[]>;
}

export interface WordForm {
  form: string;
  case?: string;
  case_name?: string;
  number?: string;
  person?: string;
  tense?: string;
  mood?: string;
  voice?: string;
  degree?: string;
  gender?: string;
  animacy?: string;
  nonfinite?: string;
  tags?: string[];
  source: string;
}

export interface LexemeDetail {
  id: number;
  lemma: string;
  pos: string;
  pos_cs?: string;
  gender?: string;
  animacy?: string;
  aspect?: string;
  ipa?: string;
  audio_url?: string;
  hyphenation?: string;
  syllables?: string[];
  etymology?: string;
  source: string;
  senses?: Sense[];
  synonyms?: Relation[];
  antonyms?: Relation[];
  related?: Relation[];
  inflection?: InflectionSummary;
  forms?: WordForm[];
  generated_forms?: string[];
  base_lemma?: string;
  translations?: Translation[];
  translations_from_lemma?: string;
}

export interface FormMatch {
  form: string;
  lexeme_id: number;
  lemma: string;
  pos: string;
  pos_cs?: string;
  case?: string;
  case_name?: string;
  number?: string;
  person?: string;
  tense?: string;
  mood?: string;
  voice?: string;
  degree?: string;
  gender?: string;
  animacy?: string;
  nonfinite?: string;
  source: string;
  tags?: string[];
}

export interface WordCardResponse {
  query: string;
  exact_lemma_match: boolean;
  ascii_fallback?: boolean;
  hyphenation?: string;
  syllables?: string[];
  form_matches?: FormMatch[];
  entries?: LexemeDetail[];
  error?: string;
}

export interface AutocompleteItem {
  lemma: string;
  pos?: string;
  pos_cs?: string;
  hyphenation?: string;
  preview?: string;
  source: string;
}

export interface AnalyzeResponse {
  query: string;
  valid: boolean;
  is_lemma: boolean;
  lemmas: string[];
  hyphenation: string;
  syllables: string[];
  morphology?: FormMatch[];
}

export interface HyphenateResponse {
  query: string;
  hyphenated: string;
  syllables: string[];
  syllable_count: number;
  source: string;
}

export interface SearchHit {
  lexeme_id: number;
  lemma: string;
  pos?: string;
  pos_cs?: string;
  definition: string;
}

export interface DataSourceInfo {
  id: string;
  name: string;
  license: string;
  url: string;
  attribution: string;
  record_count: number;
  imported_at: string;
}

export interface MetaResponse {
  service: string;
  hunspell_enabled: boolean;
  stats: {
    lexemes: number;
    word_forms: number;
    senses: number;
    relations: number;
    translations: number;
    sources: DataSourceInfo[];
  };
}
