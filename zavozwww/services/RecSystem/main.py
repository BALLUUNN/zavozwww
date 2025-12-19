from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import List, Optional
import json
import os
from russ import FilmBuddyRecommender

app = FastAPI()

BASE_DIR = os.path.dirname(os.path.abspath(__file__))

rec_service = FilmBuddyRecommender(
    ratings_file=os.path.join(BASE_DIR, 'u-1.data'),
    movies_file=os.path.join(BASE_DIR, 'u-1_russian_full.item')
)

class SearchRequest(BaseModel):
    title: Optional[str] = None
    genre: Optional[str] = None


class Grade(BaseModel):
    film_id: int
    grade: int

class MovieIdsRequest(BaseModel):
    movie_ids: List[int]

@app.post("/search")
def search_movies(request: SearchRequest):
    search_params = {
        "query": request.title if request.title else "",
        "genre": request.genre if request.genre else ""
    }
    
    result_json = rec_service.search_movies(search_params)
    return json.loads(result_json)

@app.post("/recommend")
def get_recommendations(grades: List[Grade]):

    user_history = [
        {"movie_id": g.film_id, "rating": g.grade}
        for g in grades
    ]
    
    result_json = rec_service.get_recommendations(user_history)
    return json.loads(result_json)

@app.get("/movie/{film_id}")
def get_movie_details(film_id: int):
    result_json = rec_service.get_movie_by_id(film_id)
    if result_json is None:
        raise HTTPException(status_code=404, detail="Movie not found")
    return json.loads(result_json)

@app.post("/movies/batch")
def get_movies_batch(request: MovieIdsRequest):
    result_json = rec_service.get_movies_by_ids(request.movie_ids)
    return json.loads(result_json)
