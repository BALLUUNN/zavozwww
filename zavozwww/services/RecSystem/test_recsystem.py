import unittest
import json
import pandas as pd
import os
from unittest.mock import patch, MagicMock
from fastapi.testclient import TestClient
from main import app
from russ import FilmBuddyRecommender

class TestFilmBuddyRecommender(unittest.TestCase):
    def setUp(self):
        with patch('os.path.exists', return_value=False):
            self.recommender = FilmBuddyRecommender()

    def test_initialization_synthetic(self):
        self.assertIsNotNone(self.recommender.movies_df)
        self.assertIsNotNone(self.recommender.ratings_df)
        self.assertIsNotNone(self.recommender.item_similarity_df)
        self.assertFalse(self.recommender.movies_df.empty)

    def test_search_movies_query(self):
        # Synthetic data has "Темный рыцарь"
        result = self.recommender.search_movies({"query": "Темный"})
        data = json.loads(result)
        self.assertTrue(len(data) > 0)
        self.assertIn("Темный рыцарь", [m['movie_title'] for m in data])

    def test_search_movies_genre(self):
        # Synthetic data has "Боевик"
        result = self.recommender.search_movies({"genre": "Боевик"})
        data = json.loads(result)
        self.assertTrue(len(data) > 0)
        self.assertTrue(any(m['genre'] == "Боевик" for m in data))

    def test_search_movies_genre_translation(self):
        # "action" -> "боевик"
        result = self.recommender.search_movies({"genre": "action"})
        data = json.loads(result)
        self.assertTrue(len(data) > 0)
        self.assertTrue(any(m['genre'] == "Боевик" for m in data))

    def test_get_movie_by_id_found(self):
        # ID 1 exists in synthetic data
        result = self.recommender.get_movie_by_id(1)
        self.assertIsNotNone(result)
        data = json.loads(result)
        self.assertEqual(data['movie_id'], 1)

    def test_get_movie_by_id_not_found(self):
        result = self.recommender.get_movie_by_id(99999)
        self.assertIsNone(result)

    def test_get_recommendations_empty(self):
        result = self.recommender.get_recommendations([])
        data = json.loads(result)
        # Should return popular (which is just some movies in synthetic data)
        self.assertTrue(len(data) > 0)

    def test_get_recommendations_logic(self):
        # User likes movie 1 (Drama), should get recommendations
        # In synthetic data, correlations might be random, but we check it runs
        history = [{"movie_id": 1, "rating": 5}]
        result = self.recommender.get_recommendations(history)
        data = json.loads(result)
        self.assertIsInstance(data, list)

    def test_load_data_real_files(self):
        # Mock pandas read_csv to simulate real file loading
        mock_ratings = pd.DataFrame({
            'user_id': [1, 1],
            'movie_id': [1, 2],
            'rating': [5, 4],
            'timestamp': [123, 124]
        })
        mock_movies = pd.DataFrame({
            'movie_id': [1, 2],
            'movie_title': ['Movie A (1990)', 'Movie B (1991)'],
            'release_date': ['01-Jan-1990', '01-Jan-1991'],
            'poster_url': [None, None],
            'IMDb_URL': ['url1', 'url2'],
            'description': ['Desc A', 'Desc B'],
            'unknown': [0, 0], 'Action': [1, 0], 'Adventure': [0, 0], 'Animation': [0, 0],
            'Childrens': [0, 0], 'Comedy': [0, 1], 'Crime': [0, 0], 'Documentary': [0, 0],
            'Drama': [0, 0], 'Fantasy': [0, 0], 'Film-Noir': [0, 0], 'Horror': [0, 0],
            'Musical': [0, 0], 'Mystery': [0, 0], 'Romance': [0, 0], 'Sci-Fi': [0, 0],
            'Thriller': [0, 0], 'War': [0, 0], 'Western': [0, 0]
        })

        with patch('os.path.exists', return_value=True), \
             patch('pandas.read_csv', side_effect=[mock_ratings, mock_movies]):
            
            rec = FilmBuddyRecommender()
            self.assertFalse(rec.movies_df.empty)
            # Check title cleaning
            self.assertEqual(rec.movies_df.iloc[0]['movie_title'], 'Movie A')
            # Check year extraction
            self.assertEqual(rec.movies_df.iloc[0]['year'], '1990')
            # Check genre mapping
            self.assertIn('Боевик', rec.movies_df.iloc[0]['genre'])


class TestAPI(unittest.TestCase):
    def setUp(self):
        self.client = TestClient(app)

    def test_search_endpoint(self):
        # Mocking the global rec_service in main.py is tricky because it's already initialized.
        # However, since we are running in the same process, we can patch the method on the instance.
        with patch('main.rec_service.search_movies') as mock_search:
            mock_search.return_value = json.dumps([{"movie_id": 1, "title": "Test"}])
            response = self.client.post("/search", json={"title": "Темный"})
            self.assertEqual(response.status_code, 200)
            data = response.json()
            self.assertIsInstance(data, list)
            self.assertEqual(data[0]['title'], "Test")

    def test_recommend_endpoint(self):
        with patch('main.rec_service.get_recommendations') as mock_recs:
            mock_recs.return_value = json.dumps([{"movie_id": 2, "title": "Rec"}])
            response = self.client.post("/recommend", json=[{"film_id": 1, "grade": 5}])
            self.assertEqual(response.status_code, 200)
            data = response.json()
            self.assertIsInstance(data, list)
            self.assertEqual(data[0]['title'], "Rec")

    def test_movie_details_endpoint(self):
        with patch('main.rec_service.get_movie_by_id') as mock_get:
            mock_get.return_value = json.dumps({"movie_id": 1, "title": "Test"})
            response = self.client.get("/movie/1")
            self.assertEqual(response.status_code, 200)
            self.assertEqual(response.json()['movie_id'], 1)

    def test_movie_details_404(self):
        with patch('main.rec_service.get_movie_by_id') as mock_get:
            mock_get.return_value = None
            response = self.client.get("/movie/999")
            self.assertEqual(response.status_code, 404)

if __name__ == '__main__':
    unittest.main()
