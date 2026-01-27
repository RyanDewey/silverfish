# Silverfish 🐟

**Silverfish** is a purpose-built web crawler and lead discovery engine developed as part of **Dart Ordering**, a B2B SaaS platform for restaurant online ordering and analytics.

Its primary goal is to **identify restaurants with an online presence but no modern ordering infrastructure**, extract actionable contact information, and feed those leads directly into Dart Ordering’s outbound sales workflow.

---

## ✨ Features

* 🎯 **Sales-focused lead discovery** – Finds restaurants that are strong Dart Ordering prospects
* 🌐 **Location-based crawling** – Target cities or regions for local market expansion
* 🔍 **Smart contact extraction** – Emails, phone numbers, and contact pages
* 🧠 **Noise-resistant parsing** – Regex and heuristics tuned to reduce false positives
* ⚡ **Concurrent & performant** – Designed for fast, large-scale crawling
* 📄 **CSV export for outreach** – Plug directly into Dart Ordering sales workflows

---

## 🧠 How It Works

Silverfish mirrors a real production lead pipeline:

1. **Market Targeting**
   A city or region is selected based on Dart Ordering’s expansion strategy.

2. **Website Discovery**
   Restaurant websites are discovered via seed lists, directories, or search-based entry points.

3. **Crawling & Scraping**
   Each site is crawled with controlled depth, concurrency, and rate limits.

4. **Lead Signal Extraction**
   Pages are scanned for:

   * Business contact emails
   * Phone numbers
   * Online ordering links

5. **Cleaning & Deduplication**
   Data is normalized and deduplicated to ensure high-quality leads.

6. **Sales-Ready Output**
   Results are exported as CSV files for direct ingestion into Dart Ordering’s outreach workflows.

---

## 🚀 Getting Started

### Installation

```bash
git clone https://github.com/RyanDewey/silverfish.git
cd silverfish
go mod tidy
```

### Run the Crawler

```bash
go run .
```

Results will be saved as a CSV file called `restaurants.csv`.

---

## 📊 Output Format

Each row in the CSV represents a discovered restaurant lead:

| Field        | Description               |
| ------------ | ------------------------- |
| website      | Website URL               |
| phones       | Extracted phone numbers   |
| emails       | Extracted email addresses |
| OrderingLinks| Online ordering links     |

---

## 📈 Performance & Metrics

Silverfish tracks crawl metrics such as:

* Domains crawled per second
* Requests per second
* Emails / phones extracted
* Total crawl duration

These metrics help evaluate crawl quality and optimize performance.

---

## 🧩 Use Cases

* Feeding Dart Ordering’s outbound sales pipeline
* Identifying restaurants without online ordering systems
* Local market research before city expansion
* Proof-of-concept for scalable data acquisition systems

---

## 🗺️ Roadmap

* [ ] Improved site discovery
* [ ] Better contact classification
* [ ] Distributed crawling
* [ ] Web dashboard for results

---

## 🎓 Why This Project Matters

Silverfish was built to solve a **real business problem**, not as a toy scraper:

* Demonstrates **end-to-end system design** (discovery → crawl → extract → export)
* Shows practical experience with **concurrency, performance, and reliability**
* Reflects a strong understanding of **ethical web crawling**
* Directly supports a production SaaS business (Dart Ordering)

