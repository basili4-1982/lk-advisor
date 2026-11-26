#!/usr/bin/env python3
from pathlib import Path
import pandas as pd
import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt

ROOT = Path(__file__).resolve().parent
df = pd.read_csv(ROOT / "office_cohort_ab.csv")
summary = df.groupby("arm").agg(
    match=("match", "mean"),
    preferred=("preferred_share", "mean"),
    purchases=("purchased", "mean"),
    agents=("client_id", "nunique"),
)
print(summary.to_string())

labels = ["Совпадение\nс профилем", "Доля нужных\nкатегорий", "Покупки\nна сессию"]
pop = [summary.loc["popular", "match"], summary.loc["popular", "preferred"], summary.loc["popular", "purchases"]]
hyb = [summary.loc["hybrid", "match"], summary.loc["hybrid", "preferred"], summary.loc["hybrid", "purchases"]]
x = range(len(labels))
fig, ax = plt.subplots(figsize=(10, 5.5))
ax.bar([i - 0.18 for i in x], pop, width=0.36, label="popular (контроль)", color="#c8102e")
ax.bar([i + 0.18 for i in x], hyb, width=0.36, label="hybrid + feedback", color="#1d4ed8")
for i in x:
    ax.text(i - 0.18, pop[i] + 0.02, f"{pop[i]*100:.0f}%", ha="center", fontsize=10)
    ax.text(i + 0.18, hyb[i] + 0.02, f"{hyb[i]*100:.0f}%", ha="center", fontsize=10)
ax.set_xticks(list(x), labels)
ax.set_ylim(0, 1.08)
ax.set_ylabel("доля")
ax.set_title("A/B на когорте: 10+10 агентов, 100 сессий, скрытый профиль «офисный работник»")
ax.legend()
ax.grid(axis="y", alpha=0.25)
fig.tight_layout()
out = ROOT / "office_cohort_summary.png"
fig.savefig(out, dpi=170)
print("saved", out)
