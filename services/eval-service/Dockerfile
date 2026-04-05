FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Default command — overridden by CronJob spec to select the right script
CMD ["python", "weekly_eval.py"]
