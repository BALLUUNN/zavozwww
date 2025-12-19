import pandas as pd
import numpy as np
from sklearn.metrics.pairwise import cosine_similarity
import json
import os

class FilmBuddyRecommender:
    def __init__(self, ratings_file='u.data', movies_file='u.item'):
        self.ratings_file = ratings_file
        self.movies_file = movies_file
        self.movies_df = None
        self.ratings_df = None
        self.item_similarity_df = None
        
        self.load_data()
        self.train_model()

    def load_data(self):
        genre_map = {
            'Action': 'Боевик', 'Adventure': 'Приключения', 'Animation': 'Мультфильм',
            'Children': 'Детский', 'Comedy': 'Комедия', 'Crime': 'Криминал',
            'Documentary': 'Документальный', 'Drama': 'Драма', 'Fantasy': 'Фэнтези',
            'Film-Noir': 'Нуар', 'Horror': 'Ужасы', 'Musical': 'Мюзикл',
            'Mystery': 'Мистика', 'Romance': 'Мелодрама', 'Sci-Fi': 'Фантастика',
            'Thriller': 'Триллер', 'War': 'Военный', 'Western': 'Вестерн'
        }

        if os.path.exists(self.ratings_file) and os.path.exists(self.movies_file):
            print(f"[INFO] Обнаружен dataset MovieLens. Загрузка...")

            self.ratings_df = pd.read_csv(
                self.ratings_file, 
                sep='\t', 
                names=['user_id', 'movie_id', 'rating', 'timestamp'],
                encoding='latin-1'
            )

            cols = ['movie_id', 'movie_title', 'release_date', 'poster_url',
                    'IMDb_URL', 'description', 'unknown', 'Action', 'Adventure', 'Animation',
                    'Childrens', 'Comedy', 'Crime', 'Documentary', 'Drama', 'Fantasy',
                    'Film-Noir', 'Horror', 'Musical', 'Mystery', 'Romance', 'Sci-Fi',
                    'Thriller', 'War', 'Western']
            
            raw_movies = pd.read_csv(
                self.movies_file, 
                sep='|', 
                names=cols,
                encoding='utf-8'
            )

            def get_genre_str(row):
                genres = []
                for eng_col, rus_name in genre_map.items():
                    col_key = 'Childrens' if eng_col == 'Children' else eng_col
                    if col_key in row and row[col_key] == 1:
                        genres.append(rus_name)
                return ", ".join(genres) if genres else "Разное"

            def get_year(date_str):
                if pd.isna(date_str): return ""
                try:
                    return str(date_str).split('-')[-1]
                except:
                    return ""

            def clean_title(title):
                if pd.isna(title): return ""
                return title.rsplit('(', 1)[0].strip()

            raw_movies['genre'] = raw_movies.apply(get_genre_str, axis=1)
            raw_movies['year'] = raw_movies['release_date'].apply(get_year)
            raw_movies['movie_title'] = raw_movies['movie_title'].apply(clean_title)

            self.movies_df = raw_movies[['movie_id', 'movie_title', 'genre', 'year', 'poster_url', 'description']].copy()

            self.movies_df['poster_url'] = self.movies_df['poster_url'].fillna("https://via.placeholder.com/300x450?text=No+Poster")
            self.movies_df['description'] = self.movies_df['description'].fillna("Описание отсутствует")
            
            avg_ratings = self.ratings_df.groupby('movie_id')['rating'].mean().reset_index()
            avg_ratings.columns = ['movie_id', 'rating']
            self.movies_df = pd.merge(self.movies_df, avg_ratings, on='movie_id', how='left')
            self.movies_df.set_index('movie_id', drop=False, inplace=True)

            print(f"[INFO] Загружено {len(self.ratings_df)} оценок и {len(self.movies_df)} фильмов.")
            
        else:
            print("[WARN] Файлы MovieLens (u.data, u.item) не найдены.")
            print("[INFO] Генерация синтетических данных (Русские названия)...")
            self._generate_synthetic_data()

    def _generate_synthetic_data(self):
        movies_data = [
            (1, "Побег из Шоушенка", "Драма", "1994", "https://dummy.url/1"),
            (2, "Крестный отец", "Криминал", "1972", "https://dummy.url/2"),
            (3, "Темный рыцарь", "Боевик", "2008", "https://dummy.url/3"),
            (4, "12 разгневанных мужчин", "Драма", "1957", "https://dummy.url/4"),
            (5, "Список Шиндлера", "Биография", "1993", "https://dummy.url/5"),
            (6, "Начало", "Фантастика", "2010", "https://dummy.url/6"),
            (7, "Криминальное чтиво", "Криминал", "1994", "https://dummy.url/7"),
        ]
        self.movies_df = pd.DataFrame(movies_data, columns=['movie_id', 'movie_title', 'genre', 'year', 'poster_url'])
        
        users = range(1, 101)
        ratings_list = []
        for user_id in users:
            num = np.random.randint(3, 6)
            seen = np.random.choice(self.movies_df['movie_id'], num, replace=False)
            for mid in seen:
                ratings_list.append({'user_id': user_id, 'movie_id': mid, 'rating': np.random.randint(3, 6)})
        self.ratings_df = pd.DataFrame(ratings_list)

        # Calculate average ratings
        avg_ratings = self.ratings_df.groupby('movie_id')['rating'].mean().reset_index()
        avg_ratings.columns = ['movie_id', 'rating']
        self.movies_df = pd.merge(self.movies_df, avg_ratings, on='movie_id', how='left')
        self.movies_df.set_index('movie_id', drop=False, inplace=True)

    def train_model(self):
        print("[INFO] Расчет матрицы схожести...")

        matrix = self.ratings_df.pivot_table(index='movie_id', columns='user_id', values='rating').fillna(0)
        
        similarity = cosine_similarity(matrix)
        
        self.item_similarity_df = pd.DataFrame(similarity, index=matrix.index, columns=matrix.index)
        print("[INFO] Модель готова.")
    
    def search_movies(self, search_json):
        if isinstance(search_json, str):
            params = json.loads(search_json)
        else:
            params = search_json
        
        q = params.get('query', '').lower()
        g = params.get('genre', '').lower()

        genre_translation = {
            'drama': 'драма',
            'comedy': 'комедия',
            'action': 'боевик',
            'adventure': 'приключения',
            'thriller': 'триллер',
            'horror': 'ужасы',
            'romantic-comedy': 'мелодрама',
            'western': 'вестерн',
            'animated': 'мультфильм',
            'sci-fi': 'фантастика'
        }
        
        if g in genre_translation:
            g = genre_translation[g]
        
        res = self.movies_df.copy()
        if q:
            res = res[res['movie_title'].str.lower().str.contains(q)]
        if g:
            res = res[res['genre'].str.lower().str.contains(g)]
        
        # Сортировка по рейтингу (от большего к меньшему)
        if 'rating' in res.columns:
            res = res.sort_values('rating', ascending=False)
            
        return json.dumps(res.to_dict(orient='records'), ensure_ascii=False, indent=2)

    def get_recommendations(self, user_history_json):
        if isinstance(user_history_json, str):
            history = json.loads(user_history_json)
        else:
            history = user_history_json

        if not history:
            return self._get_popular()

        user_ratings = {item['movie_id']: item['rating'] for item in history}

        candidates = {}
        for watched_id, rating in user_ratings.items():
            if watched_id not in self.item_similarity_df.index:
                continue
            
            # Используем центрирование рейтинга:
            # 1 -> -2, 2 -> -1, 3 -> 0, 4 -> 1, 5 -> 2
            # Это позволяет учитывать и негативные оценки
            weight = rating - 3
            
            if weight == 0:
                continue
            
            similar_movies = self.item_similarity_df[watched_id]
            for sim_id, sim_score in similar_movies.items():
                if sim_id in user_ratings: continue
                candidates[sim_id] = candidates.get(sim_id, 0) + (sim_score * weight) 

        sorted_recs = sorted(candidates.items(), key=lambda x: x[1], reverse=True)[:100]
        rec_ids = [m[0] for m in sorted_recs]
        
        if not rec_ids:
            return self._get_popular()
            
        # Сохраняем порядок сортировки
        # Фильтруем только те ID, которые есть в базе фильмов
        valid_rec_ids = [mid for mid in rec_ids if mid in self.movies_df.index]
        res = self.movies_df.loc[valid_rec_ids].copy()
        
        return json.dumps(res.to_dict(orient='records'), ensure_ascii=False, indent=2)

    def get_movie_by_id(self, movie_id):
        try:
            movie = self.movies_df.loc[movie_id]
            if isinstance(movie, pd.DataFrame):
                movie = movie.iloc[0]
            return json.dumps(movie.to_dict(), ensure_ascii=False, indent=2)
        except KeyError:
            return None

    def get_movies_by_ids(self, movie_ids):
        valid_ids = [mid for mid in movie_ids if mid in self.movies_df.index]
        movies = self.movies_df.loc[valid_ids]
        return json.dumps(movies.to_dict(orient='records'), ensure_ascii=False, indent=2)

    def _get_popular(self):
        stats = self.ratings_df.groupby('movie_id').agg({'rating': ['count', 'mean']})
        stats.columns = ['cnt', 'avg']

        top = stats[stats['cnt'] > 50].sort_values('avg', ascending=False).head(50)
        res = self.movies_df[self.movies_df['movie_id'].isin(top.index)]
        return json.dumps(res.to_dict(orient='records'), ensure_ascii=False, indent=2)

if __name__ == "__main__":
    service = FilmBuddyRecommender()
    
    print("\n--- TEST: Search 'Star' or Genre 'Фантастика' ---")
    print(service.search_movies({'query': 'Star', 'genre': 'Фантастика'}))
    
    print("\n--- TEST: Recs for user who likes Star Wars (ID 50 usually in MovieLens) ---")

    print(service.get_recommendations([{'movie_id': 45, 'rating': 4}]))
